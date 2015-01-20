Synapse Wire Protocol
=====================

Synapse uses a simple framing protocol to transmit data. This document describes 
the fundamental structure of the protocol, although certain details have yet to be 
formalized.

## Lead Frame

Every message on the wire begins with a "lead frame," which is formatted 
as follows:
	
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