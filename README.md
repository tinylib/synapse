Synapse
========

Synapse is a high-performance, network-protocol-agnostic, asynchronous RPC-ish framework for the Go programming language.

## Goals

Synapse is designed to make it easy to write network applications that have request-response semantics, much 
like HTTP and (some) RPC protocols. Like `net/rpc`, synapse can operate over most network protocols (or any 
`net.Conn`), and, like `net/http`, provides a standardized way to write middlewares and routers around services. 
As an added bonus, synapse has a much smaller per-request and per-connection memory footprint than `net/rpc` or 
`net/http`.

As a motivating example, let's consider a "hello world" program. (You can find the complete files in `_examples/hello_world/`.)

#### Hello World

Here's what the client code looks like:

```go
func main() {
	// This sets up a TCP connection to
	// localhost:7000 and attaches a client
	// to it. Client creation fails if it
	// can't ping the server on the other
	// end.
	client, err := synapse.DialTCP("localhost:7000")
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
```

## Suported Protocols

The following protocols are explicitly tested and
explicitly supported:

 - TCP
 - UDP
 - Unix sockets

Additionally, you can create clients and servers that use anything that satisfies the `net.Conn` interface.

Synapse does request multiplexing behind the scenes, so it generally isn't necessary to use more than one connection between the client and the server.

## Performance, etc.

Client/server throughput is determined by a number of factors, but some important ones to consider are 
latency (round-trip time, or RTT), message size, and window size. For instance, TCP has a default window size of 65,535 bytes, which 
means that your operating system kernel will not send more than that many bytes to the server without an acknowledgement.
If, for example, your average message size was 655 bytes, you could send up to 100 messages without any 
acknowledgement, but the 101st message would be blocked waiting on an ACK packet from
the server, which would take about your average RTT to arrive on your end. UDP transport, on the other hand, 
has no built-in request-response semantics, and thus no concept of "window size." Thus, in high-latency networks, using 
packet-oriented protocols like UDP can provide pretty substantial throughput gains, although ultra-high-througput use 
cases over UDP will cause requests and responses to be destroyed by packet congestion. In any case, it is important to consider 
the mechanics of your transport mechanism when seeking to optimize transaction througput.

With all that being said, it's fairly simple to achieve transaction throughput on the order of hundreds-of-thousands-per-second with
only a handful of network connections due to the Synapse wire protocol's out-of-order message processing and Go's lightweight 
concurrency model. (As of this writing, throughput of a trivial server/handler combo over one unix socket comes in at 312,500 requests 
per second.) Additionally, both client- and server-side code have been deliberately designed to consume minimal per-request 
and per-connection resources, particularly heap space.