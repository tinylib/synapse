Synapse
========

## What is this?

Synapse is:
 
 - An RPC wire protocol base on [MessagePack](http://msgpack.org)
 - A library implementation of that protocol in go

This library was designed to work with [tinylib/msgp](http://github.com/tinylib/msgp) for serialization.

## Why not MessagePack RPC?

When we first started writing code for this library, it was supposed to be a MessagePack RPC implementation. 
However, we found that the nature of the protocol makes it difficult to recover from malformed or unexpected 
messages on the wire. The primary difference between the Synapse and MessagePack RPC protocols is that synapse
uses length-prefixed frames to indicate the complete size of the message before it is read off of the wire. This 
has some benefits:

 - Serialization is done inside handlers, not in an I/O loop. The request/response body is copied into
 a reserved chunk of memory and then serialization is done in a separate goroutine. Thus, serialization can 
 fail without interrupting other messages.
 - An unexpected or unwanted message can be *efficiently* skipped, because we do not have to 
 traverse it to know its size. Being able to quickly discard unwanted or unexpected messages improves 
 security against attacks designed to exhaust server resources.
 - Message size has a hard limit at 65535 bytes, which makes it significantly more difficult to exhaust server resources (either because of traffic anomalies or a deliberate attack.)

Additionally, synapse includes a message type (called "command") that allows client and server implementations 
to probe one another programatically, which will allows us to add features (like service discovery) as the protocol 
evolves.

## Goals

Synapse is designed to make it easy to write network applications that have request-response semantics, much 
like HTTP and (some) RPC protocols. Like `net/rpc`, synapse can operate over most network protocols (or any 
`net.Conn`), and, like `net/http`, provides a standardized way to write middlewares and routers around services. 
As an added bonus, synapse has a much smaller per-request and per-connection memory footprint than `net/rpc` or 
`net/http`.

This repository contains only the "core" of the Synapse project. Over time, we will release middlewares  
in other repositories, but our intention is to keep the core as small as possible.

#### Non-goals

Synapse is not designed for large messages (there is a hard limit at 65kB), and it does not provide strong 
ordering guarantees. At the protocol level, there is no notion of CRUD operations or any other sort of stateful 
semantics; those are features that developers should provide at the application level. The same goes for auth. All 
of these features can effectively be implemented as wrappers of the core library.

## Hello World

As a motivating example, let's consider a "hello world" program. (You can find the complete files in `_examples/hello_world/`.)

Here's what the client code looks like:

```go
func main() {
	// This sets up a TCP connection to
	// localhost:7000 and attaches a client
	// to it. Client creation fails if it
	// can't ping the server on the other
	// end. Additionally, calls will fail
	// if a response isn't received in 1000 milliseconds (one second).
	client, err := synapse.Dial("tcp", "localhost:7000", 1000)
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
	err = client.Call("hello", nil, &res)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("response from server:", string(res))
}
```

And here's the server code:

```go
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
```

## Project Status

Very alpha. Expect frequent breaking changes to the API. We're actively looking for community feedback.

## Performance, etc.

Synapse is optimized for throughput over latency; in general, synapse is designed to perform well in adverse 
(high-load/high-concurrency) conditions over simple serial conditions. There are a number of implementation 
details that influence performance:

 - De-coupling of message body serialization/de-serialization from the main read/write loops reduces the 
 amount of time spend in critical (blocking) sections, meaning that time spent blocking is both lower and 
 more consistent. Additionally, malformed handlers cannot corrupt the state of the network i/o loop.
 However, this hurts performance in the simple (serial) case, because lots of time is spent copying memory
 rather than making forward progress.
 - Opportunistic coalescing of network writes reduces system call overhead, without dramatically affecting latency.
 - The objects used to maintain per-request state are arena allocated during initialization. Practically speaking, 
 this means that synapse does 0 allocations per request on the client side, and 1 allocation on the 
 server side (for `request.Name()`).
 - `tinylib/msgp` serialization is fast, compact, and versatile.

