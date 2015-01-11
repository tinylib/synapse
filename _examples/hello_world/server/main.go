package main

import (
	"fmt"
	"github.com/philhofer/synapse"
	"net"
	"os"
)

func main() {
	// Like net/http, synapse uses
	// routers to send requests to
	// the appropriate handler(s).
	// Here we'll use the one provided
	// by the synapse package, although
	// users can write their own.
	router := synapse.NewRouter()

	// Here we're registering the "hello"
	// route with a function that logs the
	// remote address of the caller and then
	// responds with "Hello, World!"
	router.HandleFunc("hello", func(req synapse.Request, res synapse.ResponseWriter) {
		fmt.Printf("received request from client at %s; responding w/ hello world\n", req.RemoteAddr())
		res.Send(synapse.String("Hello, World!"))
	})

	// We can start up servers on either
	// net.Listeners or net.Conns. Here
	// we'll use a listener.
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Serve blocks until the
	// listener is closed.
	synapse.Serve(l, router)
}
