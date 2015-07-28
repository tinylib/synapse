package synapse

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/tinylib/msgp/msgp"
)

var (
	// both the TCP and unix socket
	// client serve the same handler,
	// so in every case we expect
	// precisely the same behavior.

	// tcp client for testing
	tcpClient *Client

	// unix socket clinet for testing
	unxClient *Client

	// only global so that we
	// can attach handlers to it
	// during tests
	rt *RouteTable

	ct testing.T
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
	var s msgp.Raw
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

func finish(c io.Closer) {
	err := c.Close()
	if err != nil {
		fmt.Println("warning:", err)
	}
	time.Sleep(1 * time.Millisecond)
}

const (
	Echo Method = iota
	Nop
	DebugEcho
)

func TestMain(m *testing.M) {

	RegisterName(Echo, "echo")
	RegisterName(Nop, "nop")
	RegisterName(DebugEcho, "debug-echo")

	rt = &RouteTable{
		Echo:      EchoHandler{},
		Nop:       NopHandler{},
		DebugEcho: Debug(EchoHandler{}, log.New(os.Stderr, "debug-echo :: ", log.LstdFlags)),
	}

	l, err := net.Listen("tcp", ":7070")
	if err != nil {
		panic(err)
	}

	go Serve(l, rt)

	ul, err := net.Listen("unix", "synapse")
	if err != nil {
		panic(err)
	}

	go Serve(ul, rt)

	tcpClient, err = Dial("tcp", ":7070", 5*time.Millisecond)
	if err != nil {
		panic(err)
	}

	unxClient, err = Dial("unix", "synapse", 5*time.Millisecond)
	if err != nil {
		panic(err)
	}

	ret := m.Run()

	// note: the unix socket
	// won't get cleaned up
	// if the client is
	// closed *after* the
	// listener, which will
	// cause subsequent tests
	// to fail to bind: "address
	// already in use"

	finish(tcpClient)
	finish(unxClient)
	finish(ul)
	finish(l)

	os.Exit(ret)
}
