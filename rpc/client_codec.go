package synapse

import (
	"errors"
	"github.com/philhofer/msgp/enc"
	"io"
	"net/rpc"
)

var (
	badParams = errors.New("bad parameters")
)

func NewClientCodec(c io.ReadWriteCloser) rpc.ClientCodec {
	return &clientCodec{conn: c}
}

// clientCodec is the single-connection
// implementation of rpc.ClientCodec
type clientCodec struct {
	conn io.ReadWriteCloser
}

func (c *clientCodec) Close() error {
	return c.conn.Close()
}

func (c *clientCodec) WriteRequest(r *rpc.Request, param interface{}) error {
	wr, ok := param.(io.WriterTo)
	if !ok {
		return badParams
	}
	return writeReq(c.conn, r, wr)
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) error {
	return readRes(c.conn, r)
}

func (c *clientCodec) ReadResponseBody(x interface{}) error {
	rf, ok := x.(io.ReaderFrom)
	if !ok {
		return badParams
	}
	_, err := rf.ReadFrom(c.conn)
	return err
}

func writeReq(w io.Writer, r *rpc.Request, wrt io.WriterTo) (err error) {
	en := enc.NewEncoder(w)
	_, err = en.WriteString(r.ServiceMethod)
	if err != nil {
		return
	}
	_, err = en.WriteUint64(r.Seq)
	if err != nil {
		return
	}
	_, err = en.WriteIdent(wrt)
	return
}

func readRes(rd io.Reader, r *rpc.Response) (err error) {
	dc := enc.NewDecoder(rd)
	r.ServiceMethod, _, err = dc.ReadString()
	if err != nil {
		return
	}
	r.Seq, _, err = dc.ReadUint64()
	return
}
