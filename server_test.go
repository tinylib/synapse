package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"net"
	"testing"
)

type testString string

func (s *testString) EncodeMsg(w io.Writer) (int, error) {
	return enc.NewEncoder(w).WriteString(string(*s))
}

func (s *testString) DecodeMsg(r io.Reader) (int, error) {
	var err error
	var n int
	var ss string
	ss, n, err = enc.NewDecoder(r).ReadString()
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

func TestEcho(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	go serveListener(l, EchoHandler{})
	defer func() {
		err := l.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	cl := newclient(conn)

	instr := testString("hello, world!")
	var outstr testString
	err = cl.Call("any", &instr, &outstr)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkEcho(b *testing.B) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	go serveListener(l, EchoHandler{})
	defer func() {
		err := l.Close()
		if err != nil {
			b.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			b.Error(err)
		}
	}()

	cl := newclient(conn)

	instr := testString("hello, world!")
	var outstr testString

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cl.Call("any", &instr, &outstr)
		if err != nil {
			b.Fatalf("Iter %d: %s", i, err)
		}
	}
}
