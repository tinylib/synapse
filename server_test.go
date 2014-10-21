package synapse

import (
	"bytes"
	"github.com/philhofer/msgp/enc"
	"io"
)

type testString string

func (s *testString) EncodeMsg(w io.Writer) (int, error) {
	return enc.NewEncoder(w).WriteString(string(*s))
}

func (s *testString) EncodeTo(en *enc.MsgWriter) (int, error) {
	return en.WriteString(string(*s))
}

func (s *testString) MarshalMsg() ([]byte, error) {
	var buf bytes.Buffer
	_, err := s.EncodeTo(enc.NewEncoder(&buf))
	return buf.Bytes(), err
}

func (s *testString) DecodeMsg(r io.Reader) (int, error) {
	dc := enc.NewDecoder(r)
	n, err := s.DecodeFrom(dc)
	enc.Done(dc)
	return n, err
}

func (s *testString) DecodeFrom(dc *enc.MsgReader) (int, error) {
	var err error
	var n int
	var ss string
	ss, n, err = dc.ReadString()
	*s = testString(ss)
	return n, err
}

func (s *testString) UnmarshalMsg(b []byte) (o []byte, err error) {
	var t string
	t, o, err = enc.ReadStringBytes(b)
	if err == nil {
		*s = testString(t)
	}
	return
}

type EchoHandler struct{}

func (e EchoHandler) ServeCall(req Request, res ResponseWriter) {
	var s testString
	err := req.Decode(&s)
	if err != nil {
		panic(err)
	}
	res.Send(&s)
}

type NopHandler struct{}

func (n NopHandler) ServeCall(req Request, res ResponseWriter) {
	err := req.Decode(nil)
	if err != nil {
		panic(err)
	}
	res.Send(nil)
}
