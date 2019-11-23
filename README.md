# SimpleLB

Simple LB is the simplest Load Balancer ever created.
Listening in port 3030

It uses RoundRobin algorithm to send requests into set of backends and support
retries too.

It also performs active cleaning and passive recovery for unhealthy backends.

# Simple server

Spawns multiple backend servers.
Take input for listening port number
Default port is 3031

# How to run

Spawn four backend servers

```bash
go run .\cmd\simpleserver\server.go -port=3031
go run .\cmd\simpleserver\server.go -port=3032
go run .\cmd\simpleserver\server.go -port=3033
go run .\cmd\simpleserver\server.go -port=3034
```

Start load-balancer

```bash
go run .\cmd\simplelb\lb.go --backends=http://localhost:3031,http://localhost:3032,http://localhost:3033,http://localhost:3034
```


For more details :
Check this awesome blog-post
https://kasvith.github.io/posts/lets-create-a-simple-lb-go/