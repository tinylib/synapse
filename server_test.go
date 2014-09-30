package synapse

import (
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

type EchoHandler struct{}

func (e EchoHandler) ServeCall(req Request, res ResponseWriter) {
	var s testString
	err := req.Decode(&s)
	if err != nil {
		panic(err)
	}
	res.Send(&s)
}
