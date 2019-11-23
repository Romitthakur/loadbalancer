package main

import (
	"flag"
	"log"
	"strings"
	"net"
	"net/url"
	"net/http/httputil"
	"net/http"
	"fmt"
	"time"
	"sync/atomic"
	"sync"
	"context"
)

const (
	Attempt int = iota
	Retry
)

type Backend struct {
	URL *url.URL
	Alive bool
	ReverseProxy *httputil.ReverseProxy
	mutex sync.RWMutex
}

func (b *Backend) SetAlive(alive bool) {
	b.mutex.Lock()
	b.Alive = alive
	b.mutex.Unlock()
}

func (b *Backend) IsAlive() (alive bool) {
	b.mutex.RLock()
	alive = b.Alive
	b.mutex.RUnlock()
	return
}

type ServerPool struct {
	backends []*Backend
	currentIndex uint64
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.backends = append(s.backends, backend)
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.currentIndex, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool){
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			return
		}
	}
}

func (s *ServerPool) GetNextPeer() *Backend{
	next := s.NextIndex()
	limit := next + len(s.backends)

	for i := next; i < limit; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.currentIndex, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

// Health check for backends and update the status
func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		status := "up"
		if isBackendAlive(b.URL) {
			b.SetAlive(true)
		}else {
			b.SetAlive(false)
			status = "down"
		}
		log.Printf("%s [%s]\n", b.URL, status)
	}
}

// isBackendAlive checks if backend is alive by establishing a TCP connection and then closing the connection on result
func isBackendAlive(url *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", url.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	_ = conn.Close()
	return true
}

// HealthCheck runs serverpool health check every 20 seconds
func healthCheck() {
	serverPool.HealthCheck() // First is forced call

	t := time.NewTicker(time.Second * 20)
	for {
		select {
		case <-t.C:
			log.Println("Starting Health check...")
			serverPool.HealthCheck()
			log.Println("Health Check completed.")
		}
	}
}

// GetRetryFromContext return current retry count for given request
func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempt, ok := r.Context().Value(Attempt).(int); ok {
		return attempt
	}
	return 0
}

func lb(w http.ResponseWriter, r *http.Request){

	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()

	if peer != nil {
		fmt.Println("Proxying request to backend server: ", peer.URL)
		peer.ReverseProxy.ServeHTTP(w, r)
		return 
	}
	http.Error(w, "Service Not Available", http.StatusServiceUnavailable)
}

var serverPool ServerPool

/*
https://kasvith.github.io/posts/lets-create-a-simple-lb-go/
https://github.com/kasvith/simplelb/blob/master/main.go
// Postman stress testing
*/

func main(){
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Load balanced backends, user comma to seperate")
	flag.IntVar(&port, "port", 3030, "Load balancer Port to serve")

	flag.Parse()
	log.SetPrefix("main.go ")
	log.Println(serverList, " ", port)

	if len(serverList) == 0 {
		log.Fatal("Please provide one or more backends to load balance")
	}

	urls := strings.Split(serverList, ",")

	for _, tok := range urls {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal(err)
		}
		//log.Println(serverUrl)

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		// Implementing error handling funtion using closures, If any error occurs
		// Retry logic will kicks in and if retrying for 3 times fails, mark backend as dead
		// and route request to next backend.
		// Also, adding logic for number of attempts. If request fails for 3 attempts. Send error
		// response to client

		proxy.ErrorHandler =  func (w http.ResponseWriter, r *http.Request, e error){
			log.Printf("[%s] %s\n", serverUrl.Host, e.Error())
			retries := GetRetryFromContext(r)

			if retries > 3 {
				select {
				case <- time.After(10 * time.Millisecond):
					ctx := context.WithValue(r.Context(), Retry, retries + 1)
					proxy.ServeHTTP(w, r.WithContext(ctx))
				}
				return 
			}
			// After 3 retries mark backend down
			serverPool.MarkBackendStatus(serverUrl, false)

			// Send this request to another backend, increment attempt count
			// Handle attempts count limit in lb request handler function
			attempts := GetAttemptsFromContext(r)
			log.Printf("%s (%s) Attempting request to another backend %d\n", r.RemoteAddr, r.URL.Path, attempts)
			ctx := context.WithValue(r.Context(), Attempt, attempts+1)
			lb(w, r.WithContext(ctx))
		}

		serverPool.AddBackend(&Backend{
			URL: serverUrl,
			Alive: true,
			ReverseProxy: proxy,
		})
		log.Printf("Configured server: %s\n", serverUrl)
	}

	// create http server
	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}

	go healthCheck()

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}