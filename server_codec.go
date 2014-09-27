package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"net/rpc"
)

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
	rd, ok := x.(io.ReaderFrom)
	if !ok {
		return badParams
	}
	_, err := rd.ReadFrom(s.conn)
	return err
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

func writeRes(w io.Writer, hdr *rpc.Response, body io.WriterTo) (err error) {
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
