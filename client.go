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
	c.lr = io.LimitedReader{R: c.conn, N: 0}
	c.en = enc.NewEncoder(&c.buf)
	c.dc = enc.NewDecoder(&c.lr)
	return c
}

type clientconn struct {
	conn    io.ReadWriteCloser
	buf     bytes.Buffer
	lr      io.LimitedReader
	scratch [4]byte
	en      *enc.MsgWriter
	dc      *enc.MsgReader
}

func (c *clientconn) Close() error {
	return c.conn.Close()
}

func (c *clientconn) Call(name string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	c.buf.Reset()
	// save 4 leading bytes
	// in the buffer for size
	c.buf.Write(c.scratch[:])

	// write message to buffer
	c.en.WriteString(name)
	if in != nil {
		c.en.WriteIdent(in)
	} else {
		c.en.WriteMapHeader(0)
	}

	// write body
	isz := c.buf.Len()
	bts := c.buf.Bytes()
	binary.BigEndian.PutUint32(bts, uint32(isz))
	_, err := c.conn.Write(bts)
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
	c.lr.N = int64(osz)

	// read status code
	var code int
	code, _, err = c.dc.ReadInt()
	if err != nil {
		c.conn.Close()
		return err
	}
	status := Status(code)
	if status != OK {
		if c.lr.N > 0 {
			// empty the reader
			_, err = io.Copy(ioutil.Discard, &c.lr)
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
