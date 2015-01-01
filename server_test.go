package synapse

import (
	"github.com/philhofer/msgp/msgp"
)

type testString string

func (s *testString) MarshalMsg(b []byte) ([]byte, error) {
	return msgp.AppendString(b, string(*s)), nil
}

func (s *testString) UnmarshalMsg(b []byte) (o []byte, err error) {
	var t string
	t, o, err = msgp.ReadStringBytes(b)
	if err != nil {
		return o, err
	}
	*s = testString(t)
	return o, nil
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
