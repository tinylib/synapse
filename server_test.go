package synapse

import (
	"github.com/tinylib/msgp/msgp"
)

type testData []byte

func (s *testData) MarshalMsg(b []byte) ([]byte, error) {
	return msgp.AppendBytes(b, []byte(*s)), nil
}

func (s *testData) UnmarshalMsg(b []byte) (o []byte, err error) {
	var t []byte
	t, o, err = msgp.ReadBytesBytes(b, []byte(*s))
	*s = testData(t)
	return
}

type EchoHandler struct{}

func (e EchoHandler) ServeCall(req Request, res ResponseWriter) {
	var s testData
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
