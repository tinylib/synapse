package synapse

import (
	"bufio"
	"errors"
	"github.com/philhofer/msgp/enc"
	"io"
	"net/rpc"
	"sync"
)

var (
	badParams = errors.New("bad parameters")
)

func NewClientCodec(c io.ReadWriteCloser) rpc.ClientCodec {
	return &clientCodec{
		rbuf: bufio.NewReader(c),
		wbuf: bufio.NewWriter(c),
		conn: c,
	}
}

// clientCodec is the single-connection
// implementation of rpc.ClientCodec
type clientCodec struct {
	rbuf *bufio.Reader
	wbuf *bufio.Writer
	lock sync.Mutex
	conn io.ReadWriteCloser
}

func (c *clientCodec) Close() error {
	return c.conn.Close()
}

func (c *clientCodec) WriteRequest(r *rpc.Request, param interface{}) error {
	wr, ok := param.(enc.MsgEncoder)
	if !ok {
		return badParams
	}
	return writeReq(c.conn, r, wr)
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) error {
	return readRes(c.rbuf, r)
}

func (c *clientCodec) ReadResponseBody(x interface{}) error {
	rf, ok := x.(enc.MsgDecoder)
	if !ok {
		return badParams
	}
	_, err := rf.DecodeMsg(c.rbuf)
	return err
}

func writeReq(w *bufio.Writer, r *rpc.Request, wrt enc.MsgEncoder) (err error) {
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
	if err != nil {
		return
	}
	err = w.Flush
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
