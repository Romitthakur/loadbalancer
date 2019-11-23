package main

import (
	"fmt"
	"net/http"
	"flag"
	"log"
)

// ping request will return pong
func ping(w http.ResponseWriter, r *http.Request){
	fmt.Println("Ping request called on port: ", port)
	fmt.Fprintf(w, "Pong\n")
}

// Handle all requests
func hello(w http.ResponseWriter, r *http.Request){
	fmt.Println("Hello request called on port: ", port)
	fmt.Fprintf(w, "Hello World\n")
}

var port int

func main() {

	flag.IntVar(&port, "port", 3031, "Server port to serve")
	flag.Parse()

	http.HandleFunc("/ping", ping)
	//http.HandleFunc("/", hello)

	fmt.Println("Starting server on port: ", port)

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}