package synapse

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/philhofer/fwd"
	"github.com/tinylib/msgp/msgp"
)

const (
	// leadSize is the size of a "lead frame"
	leadSize = 11

	// maxMessageSize is the maximum size of a message
	maxMessageSize = math.MaxUint16
)

// All writes (on either side) need to be atomic; conn.Write() is called exactly once and
// should contain the entirety of the request (client-side) or response (server-side).
//
// In principle, the client can operate on any net.Conn, and the
// server can operate on any net.Listener.

// Serve starts a server on 'l' that serves
// the supplied handler. It blocks until the
// listener closes.
func Serve(l net.Listener, h Handler) error {
	s := server{l, h}
	return s.serve()
}

// ListenAndServeTLS acts identically to ListenAndServe, except that
// it expects connections over TLS1.2 (see crypto/tls). Additionally,
// files containing a certificate and matching private key for the
// server must be provided. If the certificate is signed by a
// certificate authority, the certFile should be the concatenation of
// the server's certificate followed by the CA's certificate.
func ListenAndServeTLS(network, laddr string, certFile, keyFile string, h Handler) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	l, err := tls.Listen(network, laddr, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		return err
	}
	s := server{l, h}
	return s.serve()
}

// ListenAndServe opens up a network listener
// on the provided network and local address
// and begins serving with the provided handler.
// ListenAndServe blocks until there is a fatal
// listener error.
func ListenAndServe(network string, laddr string, h Handler) error {
	l, err := net.Listen(network, laddr)
	if err != nil {
		return err
	}
	s := server{l, h}
	return s.serve()
}

// ServeConn serves an individual network
// connection. It blocks until the connection
// is closed.
func ServeConn(c net.Conn, h Handler) {
	ch := connHandler{conn: c, h: h}
	ch.connLoop()
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
		go ServeConn(conn, s.h)
	}
}

func readFrame(lead [leadSize]byte) (seq uint64, ft fType, sz int) {
	seq = (uint64(lead[0]) << 56) | (uint64(lead[1]) << 48) |
		(uint64(lead[2]) << 40) | (uint64(lead[3]) << 32) |
		(uint64(lead[4]) << 24) | (uint64(lead[5]) << 16) |
		(uint64(lead[6]) << 8) | (uint64(lead[7]))
	ft = fType(lead[8])
	sz = int((uint16(lead[9]) << 8) | uint16(lead[10]))
	return
}

func putFrame(bts []byte, seq uint64, ft fType, sz int) {
	bts[0] = byte(seq >> 56)
	bts[1] = byte(seq >> 48)
	bts[2] = byte(seq >> 40)
	bts[3] = byte(seq >> 32)
	bts[4] = byte(seq >> 24)
	bts[5] = byte(seq >> 16)
	bts[6] = byte(seq >> 8)
	bts[7] = byte(seq)
	bts[8] = byte(ft)
	usz := uint16(sz)
	bts[9] = byte(usz >> 8)
	bts[10] = byte(usz)
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
	brd := fwd.NewReaderSize(c.conn, 4096)
	remote := c.conn.RemoteAddr()

	var lead [leadSize]byte
	var seq uint64
	var sz int
	var frame fType
	for {
		// loop:
		//  - read seq, type, sz
		//  - call handler asynchronously

		_, err := brd.ReadFull(lead[:])
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("server: closing connection (error): %s", err)
				c.conn.Close()
				break
			}
			return
		}
		seq, frame, sz = readFrame(lead)

		// handle commands
		if frame == fCMD {
			var body []byte // command body; may be nil
			var cmd command // command byte

			// the 1-byte body case
			// is pretty common for fCMD
			if sz == 1 {
				var bt byte
				bt, err = brd.ReadByte()
				cmd = command(bt)
			} else {
				body = make([]byte, sz)
				_, err = brd.ReadFull(body)
				cmd = command(body[0])
				body = body[1:]
			}
			if err != nil {
				log.Printf("synapse server: closing connection (error): %s", err)
				c.conn.Close()
				return
			}
			go handleCmd(c.conn, seq, cmd, body)
			continue
		}

		// the only valid frame
		// type left is fREQ
		if frame != fREQ {
			_, err := brd.Skip(sz)
			if err != nil {
				log.Printf("synapse server: closing connection (error): %s", err)
				c.conn.Close()
				return
			}
			continue
		}

		w := wrappers.pop(c.conn)

		if cap(w.in) >= sz {
			w.in = w.in[0:sz]
		} else {
			w.in = make([]byte, sz)
		}

		// don't block forever here,
		// deadline if we haven't already
		// buffered the connection data
		deadline := false
		if brd.Buffered() < sz {
			deadline = true
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		}
		_, err = brd.ReadFull(w.in)
		if err != nil {
			log.Printf("synapse server: closing connection (error): %s", err)
			c.conn.Close()
			break
		}
		// clear read deadline
		if deadline {
			c.conn.SetReadDeadline(time.Time{})

		}

		// trigger handler
		w.seq = seq
		go handleReq(w, remote, c.h)
	}
}

// connWrapper contains all the resources
// necessary to execute a Handler on a request
type connWrapper struct {
	next *connWrapper // only used by slab
	seq  uint64       // sequence number
	in   []byte       // incoming message
	req  request
	res  response
	conn io.Writer
}

// handleconn sets up the Request and ResponseWriter
// interfaces and calls the handler.
// output is written to cw.parent.conn
func handleReq(cw *connWrapper, remote net.Addr, h Handler) {
	// clear/reset everything
	cw.req.addr = remote
	cw.res.wrote = false

	var err error
	cw.req.name, cw.req.in, err = msgp.ReadStringBytes(cw.in)
	if err != nil {
		cw.res.Error(BadRequest)
	} else {
		h.ServeCall(&cw.req, &cw.res)
		// if the handler didn't write a body
		if !cw.res.wrote {
			cw.res.Send(nil)
		}
	}

	blen := len(cw.res.out) - leadSize // length minus frame length
	putFrame(cw.res.out, cw.seq, fRES, blen)

	_, err = cw.conn.Write(cw.res.out)
	if err != nil {
		// TODO: print something more usefull...?
		log.Printf("synapse server: error writing response: %s", err)
	}
	wrappers.push(cw)
}

func handleCmd(w io.WriteCloser, seq uint64, cmd command, body []byte) {
	var (
		buf  bytes.Buffer
		lead [leadSize + 1]byte // one extra, b/c we always write cmd
		res  []byte
	)

	lead[leadSize] = byte(cmd)

	act := cmdDirectory[cmd]
	var err error
	if act == nil {
		lead[leadSize] = byte(cmdInvalid)
	} else {
		res, err = act.Server(w, body)
		if err != nil {
			lead[leadSize] = byte(cmdInvalid)
		}
	}

	sz := len(res) + 1
	putFrame(lead[:], seq, fCMD, sz)

	var bts []byte

	if sz == 1 {
		bts = lead[:]
	} else {
		buf.Write(lead[:])
		buf.Write(res)
		bts = buf.Bytes()
	}

	_, err = w.Write(bts)
	if err != nil {
		log.Printf("synapse server: error: %s", err)
	}
}
