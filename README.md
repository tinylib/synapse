Synapse
========

Synapse is a high-performance, network-protocol-agnostic, asynchronous RPC-ish framework for the Go programming language.

## Goals

Synapse is designed to make it easy to write network applications that have request-response semantics, much 
like HTTP and (some) RPC protocols. Like `net/rpc`, synapse can operate over most network protocols (or any 
`net.Conn`), and, like `net/http`, provides a standardized way to write middlewares and routers around services. 
As an added bonus, synapse has a much smaller per-request and per-connection memory footprint than `net/rpc` or 
`net/http`.

#### Non-goals

Synapse is not intended to be a replacement for industrial-grade message queue solutions like ZeroMQ, RabbitMQ, etc. 
Rather, synapse provides a convenient way to bind to a network connection without worrying about message framing, 
multiplexing, serialization, pipelining, and so on. As synapse matures, we may release middlewares and other sorts 
of plugins for synapse in other repositories, but our intention is to keep the core as simple as possible.

As a motivating example, let's consider a "hello world" program. (You can find the complete files in `_examples/hello_world/`.)

## Hello World

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

## Suported Protocols

The following protocols are explicitly supported:

 - TCP
 - TLS
 - Unix sockets

Additionally, you can create clients and servers that use anything that satisfies the `net.Conn` interface on 
both ends.

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
 this means that, typically, synapse does 0 allocations per request on the client side, and 1 allocation on the 
 server side (for `request.Name()`).
 - `tinylib/msgp` serialization is fast, compact, and versatile.

