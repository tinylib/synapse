package main

import (
	"github.com/philhofer/synapse"
	"log"
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
		log.Println("received request from client at", req.RemoteAddr())
		res.Send(synapse.String("Hello, World!"))
	})

	// ListenAndServe blocks forever
	// serving the provided handler.
	log.Fatalln(synapse.ListenAndServe("tcp", "localhost:7000", router))
}
