package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"log"
	"math"
	"net"
	"time"
)

// maxFRAMESIZE is the maximum size of a message
const maxFRAMESIZE = math.MaxUint16

// FRAMING EXPLANATION:
//
// Request:
//   |   seq#  |   len   |   name  |  msg  |
//   | uint64  |  uint32 |  string | (bts) |
//
// Response:
//   |   seq#  |   len   |   code  |  msg  |
//   | uint64  |  uint32 |  int64  | (bts) |
//
// - 'seq' should increase monotonically per request
// - 'len' should be the number of bytes *remaining* to be read
// - 'name' is a messagepack-encoded string
// - 'code' is a messagepack-encoded signed integer (up to 64 bits)
// - 'msg' is any messagepack-encoded message
//
// Clients and servers do asynchronous writes and synchronous reads.
//
// All writes (on either side) need to be atomic; conn.Write() is called exactly once and
// (should) contain the entirety of the request (client-side) or response (server-side).
//
// Clients ignore requests with bad sequence numbers.
//
// Both clients and servers force-close connections if they
// detect a frame larger than maxFRAMESIZE. This is to make it
// impossible for one side to ask the other (for whatever reason)
// to allocate an unreasonable amount of memory. Also, the maximum
// frame size is the default size of the TCP window, which keeps
// latency reasonable. (No more than one ACK per write.)
//
// In principle, the client can operate on any net.Conn, and the
// server can operate on any net.Listener. However, the protocol
// was designed with TCP in mind.

// Serve starts a server on 'l' that serves
// the supplied handler. It blocks until the
// handler closes.
func Serve(l net.Listener, h Handler) error {
	s := server{l, h}
	return s.serve()
}

type server struct {
	l net.Listener
	h Handler
}

func (s *server) serve() error {
	for {
		conn, err := s.l.Accept()
		if err != nil {
			return err
		}
		ch := &connHandler{conn: conn, h: s.h}
		go ch.connLoop()
	}
}

// connHandler handles network
// connections and multiplexes requests
// to connWrappers
type connHandler struct {
	h    Handler
	conn net.Conn
}

// connLoop continuously polls the connection.
// requests are read synchronously; the responses
// are written in a spawned goroutine
func (c *connHandler) connLoop() {
	brd := bufio.NewReader(c.conn)

	var lead [12]byte
	var seq uint64
	var sz uint32
	for {
		// loop:
		//  - read seq and sz
		//  - call handler asynchronously

		_, err := io.ReadFull(brd, lead[:])
		if err != nil {
			if err != io.EOF {
				log.Printf("server: fatal: %s", err)
			}
			c.conn.Close()
			break
		}
		seq = binary.BigEndian.Uint64(lead[0:8])
		sz = binary.BigEndian.Uint32(lead[8:12])

		// reject frames
		// larger than 65kB
		if sz > maxFRAMESIZE {
			log.Printf("synapse server: client at %s sent a %d-byte frame; force-closing connection...",
				c.conn.RemoteAddr().String(), sz)
			c.conn.Close()
			break
		}

		isz := int(sz)
		w := popWrapper(c)

		if cap(w.in) >= isz {
			w.in = w.in[0:isz]
		} else {
			w.in = make([]byte, isz)
		}

		// don't block forever here,
		// deadline if we haven't already
		// buffered the connection data
		deadline := false
		if brd.Buffered() < isz {
			deadline = true
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		}
		_, err = io.ReadFull(brd, w.in)
		if err != nil {
			log.Printf("synapse server: fatal error: %s", err)
			c.conn.Close()
			break
		}
		// clear read deadline
		if deadline {
			c.conn.SetReadDeadline(time.Time{})

		}

		// trigger handler
		w.seq = seq
		w.dc.Reset(bytes.NewReader(w.in))
		go handleConn(w, c.h) // must write atomically
	}
}

// connWrapper contains all the resources
// necessary to execute a Handler on a request
type connWrapper struct {
	seq    uint64
	lead   [12]byte
	in     []byte       // incoming message
	out    bytes.Buffer // we need to write to parent.conn atomically, so buffer the whole message
	req    request
	res    response
	en     *enc.MsgWriter // wraps bytes.Buffer
	dc     *enc.MsgReader // wraps 'in'
	parent *connHandler
}

// handleconn sets up the Request and ResponseWriter
// interfaces and calls the handler.
// output is written to cw.parent.conn
func handleConn(cw *connWrapper, h Handler) {
	// clear/reset everything
	cw.out.Reset()
	cw.req.dc = cw.dc
	cw.req.addr = cw.parent.conn.RemoteAddr()
	cw.res.en = cw.en
	cw.res.err = nil
	cw.res.wrote = false
	cw.res.status = Invalid

	// reserve 12 bytes at the beginning
	cw.out.Write(cw.lead[:])

	var err error
	cw.req.name, _, err = cw.dc.ReadString()
	if err != nil {
		cw.res.WriteHeader(BadRequest)
	} else {
		h.ServeCall(&cw.req, &cw.res)
	}

	if !cw.res.wrote {
		cw.res.WriteHeader(OK)
		cw.res.Send(nil)
	}

	bts := cw.out.Bytes()
	blen := len(bts) - 12 // length minus frame length
	binary.BigEndian.PutUint64(bts[0:8], cw.seq)
	binary.BigEndian.PutUint32(bts[8:12], uint32(blen))

	// TODO: timeout?
	// net.Conn takes care of the
	// locking for us
	_, err = cw.parent.conn.Write(bts)
	if err != nil {
		// TODO: print something more usefull...?
		log.Printf("synapse server: error writing response: %s", err)
	}
	pushWrapper(cw)
}
