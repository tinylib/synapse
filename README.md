Synapse
========

Synapse is a high-performance, network-protocol-agnostic, asynchronous RPC framework for the Go programming language.



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