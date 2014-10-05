Synapse Wire Protocol
=====================

Synapse uses a simple framing protocol in order to allow requests and
responses to be handled without blocking the connection. The bodies of most
messages are formatted using the [MessagePack](http://msgpack.org) serialization
protocol.

## Lead Frame

Every message on the wire begins with a "lead frame," which is formatted 
as follows.
	
	+-----------------+------------+----------------+==========+
	| Sequence Number | Frame Type | Message Length |   DATA   |
	+-----------------+------------+----------------+==========+
	|    (8 bytes)    |  (1 byte)  |    (2 bytes)   |  N bytes |
	+-----------------+------------+----------------+==========+
	| 64-bit usigned  |   uint8    | 16-bit unsigned|          |
	| integer, big-   |            | integer, big-  |          |
	| endian          |            | endian         |          |
    +-----------------+------------+----------------+==========+

The sequence number is a per-connection request counter that is incremented
atomically on the client side. It is used to uniquely identify each request. Responses
have the same sequence number as their corresponding request.

The frame types are defined as follows:

| Value | Type |
|:-----:|:----:|
| 0 | Invalid |
| 1 | Request |
| 2 | Response |
| 3 | Command |

The message length is the number of bytes in the message *not including the lead frame.*

## Request Message

|   | Name | Message |
|:-:|:----:|:-------:|
| Value | MessagePack string | MessagePack object |

## Response Message

|   | Code | Message |
|:-:|:----:|:-------:|
| Value | MessagePack int | MessagePack object |

## Command Message

|   | Type | Body |
|:-:|:----:|:----:|
| Value | byte | N-1 bytes |

TODO: document commands