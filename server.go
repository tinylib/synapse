package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"net"
)

func serveConn(c net.Conn, h Handler) {
	rd := bufio.NewReader(c)
	wr := bufio.NewWriter(c)

	var outbuf bytes.Buffer // output buffer; we have to know size in advance

	var lead [4]byte // for msg size
	var sz uint32    // ""
	var req request
	var res response

	req.addr = c.RemoteAddr().String()
	res.en = enc.NewEncoder(&outbuf)

	// this is the limited reader that
	// the req.decoder uses
	lr := io.LimitedReader{R: rd, N: 0}
	req.dc = enc.NewDecoder(&lr)

	// Loop:
	//  - read big-endian uint32 msg size
	//  - copy (sz) bytes into 'inbuf'
	//  - call handler
	//  - handler writes response to 'outbuf'
	//  - write response length; copy buffer to conn
	for {
		outbuf.Reset()

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
		// wasn't written
		if !res.wrote {
			res.WriteHeader(OK)
			res.Send(nil)
		}

		ol := outbuf.Len()
		binary.BigEndian.PutUint32(lead[:], uint32(ol))
		_, err = wr.Write(lead[:])
		if err != nil {
			c.Close()
			break
		}
		_, err = wr.Write(outbuf.Bytes())
		if err != nil {
			c.Close()
			break
		}
		err = wr.Flush()
		if err != nil {
			c.Close()
			break
		}
	}
}
