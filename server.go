package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"strings"
	"time"
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
// handler closes.
func Serve(l net.Listener, h Handler) error {
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

	var lead [leadSize]byte
	var seq uint64
	var sz uint16
	for {
		// loop:
		//  - read seq, type, sz
		//  - call handler asynchronously

		_, err := io.ReadFull(brd, lead[:])
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("server: fatal: %s", err)
				c.conn.Close()
				break
			}
			return
		}
		seq = binary.BigEndian.Uint64(lead[0:8])
		frame := fType(lead[8])
		sz = binary.BigEndian.Uint16(lead[9:11])
		isz := int(sz)

		// handle commands
		if frame == fCMD {
			var body []byte // command body; may be nil
			var cmd command // command byte

			// the 1-byte body case
			// is pretty common for fCMD
			if isz == 1 {
				var bt byte
				bt, err = brd.ReadByte()
				cmd = command(bt)
			} else {
				body = make([]byte, isz)
				_, err = io.ReadFull(brd, body)
				cmd = command(body[0])
				body = body[1:]
			}
			if err != nil {
				log.Printf("synapse server: fatal: %s", err)
				c.conn.Close()
				return
			}
			go handleCmd(c.conn, seq, cmd, body)
			continue
		}

		// the only valid frame
		// type left is fREQ
		if frame != fREQ {
			io.CopyN(ioutil.Discard, brd, int64(sz))
			continue
		}

		w := popWrapper(c.conn)

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
		go handleReq(w, c.conn.RemoteAddr(), c.h)
	}
}

// connWrapper contains all the resources
// necessary to execute a Handler on a request
type connWrapper struct {
	seq  uint64
	lead [leadSize]byte
	in   []byte       // incoming message
	out  bytes.Buffer // we need to write to parent.conn atomically, so buffer the whole message
	req  request
	res  response
	en   *enc.MsgWriter // wraps bytes.Buffer
	dc   *enc.MsgReader // wraps 'in'
	conn io.Writer
}

// handleconn sets up the Request and ResponseWriter
// interfaces and calls the handler.
// output is written to cw.parent.conn
func handleReq(cw *connWrapper, remote net.Addr, h Handler) {
	// clear/reset everything
	cw.out.Reset()
	cw.req.dc = cw.dc
	cw.req.addr = remote
	cw.res.en = cw.en
	cw.res.wrote = false
	cw.res.status = Invalid

	// reserve 13 bytes at the beginning
	cw.out.Write(cw.lead[:])

	var err error
	cw.req.name, _, err = cw.dc.ReadString()
	if err != nil {
		cw.res.WriteHeader(BadRequest)
	} else {
		h.ServeCall(&cw.req, &cw.res)
		if !cw.res.wrote {
			cw.res.WriteHeader(OK)
			cw.res.Send(nil)
		}
	}

	bts := cw.out.Bytes()
	blen := len(bts) - leadSize // length minus frame length
	binary.BigEndian.PutUint64(bts[0:8], cw.seq)
	bts[8] = byte(fRES)
	binary.BigEndian.PutUint16(bts[9:11], uint16(blen))

	// TODO: timeout?
	_, err = cw.conn.Write(bts)
	if err != nil {
		// TODO: print something more usefull...?
		log.Printf("synapse server: error writing response: %s", err)
	}
	pushWrapper(cw)
}

func handleCmd(w io.WriteCloser, seq uint64, cmd command, body []byte) {
	var (
		buf  bytes.Buffer
		lead [leadSize + 1]byte // one extra, b/c we always write cmd
		res  []byte
	)

	// lead 8 are always seq
	binary.BigEndian.PutUint64(lead[0:8], seq)
	lead[8] = byte(fCMD)
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

	sz := uint16(len(res)) + 1
	binary.BigEndian.PutUint16(lead[9:11], sz)

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
