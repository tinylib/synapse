package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"net"
)

func serveListener(l net.Listener, h Handler) {
	for {
		conn, err := l.Accept()
		if err != nil {
			l.Close()
			break
		}
		go serveConn(conn, h)
	}
}

func serveConn(c net.Conn, h Handler) {
	rd := bufio.NewReader(c)

	var outbuf bytes.Buffer // output buffer; we have to know size in advance

	var lead [4]byte // for msg size
	var sz uint32    // ""
	var req request  // for handler request
	var res response // for handler response

	req.addr = c.RemoteAddr().String()
	res.en = enc.NewEncoder(&outbuf)

	// this is the limited reader that
	// the req.decoder uses; we change
	// N before every handler call
	lr := io.LimitedReader{R: rd, N: 0}
	req.dc = enc.NewDecoder(&lr)

	// Loop:
	//  - read big-endian uint32 msg size
	//  - set limit on limit reader for req
	//  - call handler
	//  - handler writes response to 'outbuf'
	//  - write response length to head of buffer; write to conn
	for {
		outbuf.Reset()
		// save 4 bytes at the front of the buffer
		outbuf.Write(lead[:])

		// reset response state
		res.wrote = false
		res.status = Invalid
		res.err = nil

		_, err := io.ReadFull(rd, lead[:])
		if err != nil {
			c.Close()
			break
		}
		sz = binary.BigEndian.Uint32(lead[:])

		// TODO(philhofer): handle this better
		if sz == 0 {
			continue
		}

		// reset limit on request read
		lr.N = int64(sz)
		err = req.refresh()
		if err != nil {
			c.Close()
			break
		}

		h.ServeCall(&req, &res)

		// in case response
		// wasn't written by
		// the hanlder
		if !res.wrote {
			res.WriteHeader(OK)
			res.Send(nil)
		}

		// this is a little gross; we
		// massage the first 4 bytes of the
		// buffer by hand. but, this way
		// we do exactly one write every time
		ol := outbuf.Len()
		bts := outbuf.Bytes()
		binary.BigEndian.PutUint32(bts, uint32(ol))
		_, err = c.Write(bts)
		if err != nil {
			c.Close()
			break
		}
	}
}
