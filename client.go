package synapse

import (
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"io/ioutil"
)

func statusErr(s Status) error { return ErrStatus{s} }

type ErrStatus struct {
	Status Status
}

func (e ErrStatus) Error() string {
	switch e.Status {
	case NotFound:
		return "Not Found"
	case NotAuthed:
		return "Not Authorized"
	case ServerError:
		return "Server Error"
	case Invalid:
		return "Invalid Status"
	default:
		return "<unknown>"
	}
}

func newclient(rw io.ReadWriteCloser) *clientconn {
	c := &clientconn{
		conn: rw,
	}
	c.en = enc.NewEncoder(&c.buf)
	c.dc = enc.NewDecoder(&c.buf)
	return c
}

type clientconn struct {
	conn    io.ReadWriteCloser
	buf     bytes.Buffer
	scratch [4]byte
	en      *enc.MsgWriter
	dc      *enc.MsgReader
}

func (c *clientconn) Close() error {
	return c.conn.Close()
}

func (c *clientconn) Call(name string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	c.buf.Reset()

	// write message to buffer
	c.en.WriteString(name)
	if in != nil {
		c.en.WriteIdent(in)
	} else {
		c.en.WriteMapHeader(0)
	}

	// write leading size
	isz := c.buf.Len()
	binary.BigEndian.PutUint32(c.scratch[:], uint32(isz))
	_, err := c.conn.Write(c.scratch[:])
	if err != nil {
		c.conn.Close()
		return err
	}
	// write request body from buffer
	_, err = c.conn.Write(c.buf.Bytes())
	if err != nil {
		c.conn.Close()
		return err
	}

	// RESET
	c.buf.Reset()

	// read leading size
	_, err = io.ReadFull(c.conn, c.scratch[:])
	if err != nil {
		c.conn.Close()
		return err
	}
	osz := binary.BigEndian.Uint32(c.scratch[:])
	if osz == 0 {
		return statusErr(ServerError)
	}
	// read response body into buffer
	_, err = io.CopyN(&c.buf, c.conn, int64(osz))
	if err != nil {
		c.conn.Close()
		return err
	}
	// read status code
	var code int
	var nr int
	code, nr, err = c.dc.ReadInt()
	if err != nil {
		c.conn.Close()
		return err
	}
	status := Status(code)
	left := int64(osz) - int64(nr)
	if status != OK {
		if left > 0 {
			_, err = io.CopyN(ioutil.Discard, c.conn, left)
			if err != nil {
				c.conn.Close()
				return err
			}
		}
		return statusErr(status)
	}

	_, err = c.dc.ReadIdent(out)
	return err
}
