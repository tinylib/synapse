package synapse

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strings"
)

// ServePacket creates a server that serves
// a packet-oriented connection created by
// net.ListenPacket().
func ServePacket(c net.PacketConn, h Handler) error {
	pch := pconnHandler{c, h}
	return pch.pconnLoop()
}

// pconn is a shim to wrap
// the connection
type pconn struct {
	net.PacketConn
	remote net.Addr
}

// pconn is an io.Writer that writes the
// response packet to p.remote
func (p pconn) Write(b []byte) (int, error) {
	return p.PacketConn.WriteTo(b, p.remote)
}

type pconnHandler struct {
	conn net.PacketConn
	h    Handler
}

// like connloop, but for packet connections
func (c pconnHandler) pconnLoop() error {
	// receive buffer
	var rcv [32000]byte
	var msg []byte
	var seq uint64
	var sz uint16

	for {
		nr, remote, err := c.conn.ReadFrom(rcv[:])
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("server: fatal: %s", err)
				c.conn.Close()
				return err
			}
			return nil
		}
		msg = rcv[:nr]
		if len(msg) < 11 {
			// bad packet...?
			continue
		}

		seq = binary.BigEndian.Uint64(msg[0:8])
		frame := fType(msg[8])
		sz = binary.BigEndian.Uint16(msg[9:11])
		isz := int(sz)

		// handle commands
		if frame == fCMD {
			var body []byte // command body; may be nil

			cmd := command(msg[11])
			if isz > 1 {
				body = make([]byte, isz-1)
				copy(body[0:], msg[12:])
			}
			go handleCmd(pconn{c.conn, remote}, seq, cmd, body)
			continue
		}

		// the only valid frame
		// type left is fREQ
		if frame != fREQ {
			continue
		}

		// provide the wrapper
		// that ensures the response
		// is written to the
		// correct address
		w := popWrapper(pconn{c.conn, remote})

		if cap(w.in) >= isz {
			w.in = w.in[0:isz]
		} else {
			w.in = make([]byte, isz)
		}

		copy(w.in[0:], msg[11:])

		// trigger handler
		w.seq = seq
		w.dc.Reset(bytes.NewReader(w.in))
		go handleReq(w, remote, c.h)
	}
}
