package synapse

import (
	"bufio"
	"github.com/philhofer/msgp/enc"
	"io"
	"net/rpc"
	"sync"
)

func NewServerCode(c io.ReadWriteCloser) rpc.ServerCodec {
	return &serverCodec{
		rbuf: bufio.NewReader(c),
		wbuf: bufio.NewWriter(c),
		conn: c,
	}
}

type serverCodec struct {
	rbuf  *bufio.Reader
	wbuf  *bufio.Writer
	conn  io.ReadWriteCloser
	wlock sync.Mutex
}

func (s *serverCodec) Close() error {
	return s.conn.Close()
}

func (s *serverCodec) ReadRequestHeader(r *rpc.Request) error {
	return readReq(s.rbuf, r)
}

func (s *serverCodec) ReadRequestBody(x interface{}) error {
	rd, ok := x.(enc.MsgDecoder)
	if !ok {
		return badParams
	}
	_, err := rd.DecodeMsg(s.rbuf)
	return err
}

func (s *serverCodec) WriteResponse(r *rpc.Response, body interface{}) error {
	wt, ok := body.(enc.MsgEncoder)
	if !ok {
		return badParams
	}
	s.wlock.Lock()
	err := writeRes(s.wbuf, r, wt)
	s.wlock.Unlock()
	return err
}

func readReq(r io.Reader, req *rpc.Request) (err error) {
	dc := enc.NewDecoder(r)
	req.ServiceMethod, _, err = dc.ReadString()
	if err != nil {
		return
	}
	req.Seq, _, err = dc.ReadUint64()
	enc.Done(dc)
	return
}

func writeRes(w *bufio.Writer, hdr *rpc.Response, body enc.MsgEncoder) (err error) {
	en := enc.NewEncoder(w)
	_, err = en.WriteString(hdr.ServiceMethod)
	if err != nil {
		return
	}
	_, err = en.WriteUint64(hdr.Seq)
	if err != nil {
		return
	}
	_, err = en.WriteIdent(body)
	if err != nil {
		return
	}
	err = w.Flush()
	return
}
