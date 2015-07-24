package main

import (
	"github.com/tinylib/synapse"
	"log"
)

const (
	// Each route is simply
	// an index into a table,
	// much like a system call
	// or a file descriptor.
	// Generall, it's easiest
	// to define your routes
	// in a const-iota block.
	Hello synapse.Method = iota
)

// helloHandler will implement
// synapse.Handler
type helloHandler struct{}

func (h helloHandler) ServeCall(req synapse.Request, res synapse.ResponseWriter) {
	log.Println("received request from client with addr", req.RemoteAddr())
	res.Send(synapse.String("Hello, World!"))
}

func main() {
	// RouteTable is the simplest
	// form of request routing:
	// it matches the method number
	// to an index into the table.
	rt := synapse.RouteTable{
		Hello: helloHandler{},
	}

	// ListenAndServe blocks forever
	// serving the provided handler.
	log.Fatalln(synapse.ListenAndServe("tcp", "localhost:7000", &rt))
}
