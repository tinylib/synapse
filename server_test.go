package synapse

import (
	"io"

	"github.com/philhofer/msgp/msgp"
)

type testString string

func (s *testString) EncodeMsg(en *msgp.Writer) (int, error) {
	return len(string(*s)), en.WriteString(string(*s))
}

func (s *testString) MarshalMsg(curr []byte) ([]byte, error) {
	//	var buf *bytes.Buffer = bytes.NewBuffer(curr)
	//	_, err := buf.WriteString(string(*s))
	return msgp.AppendString(curr, string(*s)), nil
}

func (s *testString) DecodeMsg(r io.Reader) (int, error) {
	dc := msgp.NewReader(r)
	n, err := s.DecodeFrom(dc)
	return n, err
}

func (s *testString) DecodeFrom(dc *msgp.Reader) (int, error) {
	var err error
	var ss string
	ss, err = dc.ReadString()
	*s = testString(ss)
	return len(ss), err
}

func (s *testString) UnmarshalMsg(b []byte) (o []byte, err error) {
	var t string
	t, o, err = msgp.ReadStringBytes(b)
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
