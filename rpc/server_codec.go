package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"net/rpc"
)

func NewServerCode(c io.ReadWriteCloser) rpc.ServerCodec {
	return &serverCodec{conn: c}
}

type serverCodec struct {
	conn io.ReadWriteCloser
}

func (s *serverCodec) Close() error {
	return s.conn.Close()
}

func (s *serverCodec) ReadRequestHeader(r *rpc.Request) error {
	return readReq(s.conn, r)
}

func (s *serverCodec) ReadRequestBody(x interface{}) error {
	rd, ok := x.(enc.MsgDecoder)
	if !ok {
		return badParams
	}
	_, err := rd.DecodeMsg(s.conn)
	return err
}

func (s *serverCodec) WriteResponse(r *rpc.Response, body interface{}) error {
	wt, ok := body.(enc.MsgEncoder)
	if !ok {
		return badParams
	}
	return writeRes(s.conn, r, wt)
}

func readReq(r io.Reader, req *rpc.Request) (err error) {
	dc := enc.NewDecoder(r)
	req.ServiceMethod, _, err = dc.ReadString()
	if err != nil {
		return
	}
	req.Seq, _, err = dc.ReadUint64()
	return
}

func writeRes(w io.Writer, hdr *rpc.Response, body enc.MsgEncoder) (err error) {
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
	return
}
