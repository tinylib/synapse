package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"log"
	"net"
	"time"
)

// PROTOCOL:
//
// Request:
//   |   seq   |   len   |   name  |  msg  |
//   | uint64  |  uint32 |   str   | (bts) |
//
// Response:
//   |   seq   |   len   |   code  |  msg  |
//   | uint64  |  uint32 |  int64  | (bts) |
//
//
// Servers read seq+len+request synchronously and write reponses asynchronously.
// Clients read write requests asynchronously and read seq+len+response synchronously.
//
// All writes (on either side) need to be atomic; conn.Write() is called exactly once

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

type connHandler struct {
	h    Handler
	conn net.Conn
}

func (c *connHandler) connLoop() {
	brd := bufio.NewReaderSize(c.conn, 1024)

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
		isz := int(sz)
		w := popWrapper(c)

		if cap(w.in) >= isz {
			w.in = w.in[0:isz]
		} else {
			w.in = make([]byte, isz)
		}

		// don't block forever here
		c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, err = io.ReadFull(brd, w.in)
		if err != nil {
			log.Printf("server: fatal error: %s", err)
			c.conn.Close()
			break
		}
		// clear read deadline
		c.conn.SetReadDeadline(time.Time{})

		// trigger handler
		w.seq = seq
		w.dc.Reset(bytes.NewReader(w.in))
		go handleConn(w, c.h) // must write atomically
	}
}

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

		if !cw.res.wrote {
			cw.res.WriteHeader(OK)
			cw.res.Send(nil)
		}

		bts := cw.out.Bytes()
		blen := len(bts) - 12 // length minus frame length
		binary.BigEndian.PutUint64(bts[0:8], cw.seq)
		binary.BigEndian.PutUint32(bts[8:12], uint32(blen))

		// net.Conn takes care of the
		// locking for us
		_, err = cw.parent.conn.Write(bts)
		if err != nil {
			// TODO: print something usefull...?
		}
	}

	pushWrapper(cw)
}
