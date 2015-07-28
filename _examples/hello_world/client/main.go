package main

import (
	"fmt"
	"github.com/tinylib/synapse"
	"os"
	"time"
)

const (
	Hello synapse.Method = 0
)

func main() {
	// This sets up a TCP connection to
	// localhost:7000 and attaches a client
	// to it. Client creation fails if it
	// can't ping the server on the other
	// end. Additionally, calls will fail
	// if a response isn't received within 1 second.
	client, err := synapse.Dial("tcp", "localhost:7000", time.Second)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Here we make a remote call to
	// the method called "hello," and
	// we pass an object for the
	// response to be decoded into.
	// synapse.String is a convenience
	// provided for sending strings
	// back and forth.
	var res synapse.String
	err = client.Call(Hello, nil, &res)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("response from server:", string(res))
}
