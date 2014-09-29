package synapse

import (
	"github.com/inconshreveable/muxado"
	"github.com/philhofer/msgp/enc"
	"io"
	"net"
	"testing"
	"time"
)

type testString string

func (s *testString) EncodeMsg(w io.Writer) (int, error) {
	return enc.NewEncoder(w).WriteString(string(*s))
}

func (s *testString) DecodeMsg(r io.Reader) (int, error) {
	var err error
	var n int
	var ss string
	dec := enc.NewDecoder(r)
	ss, n, err = dec.ReadString()
	enc.Done(dec)
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
		l.(*net.TCPListener).SetDeadline(time.Now())
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

func TestMultiplex(t *testing.T) {
	l, err := muxado.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go serveMuxListener(l, EchoHandler{})

	// wait for listener to start up
	time.Sleep(5 * time.Microsecond)

	conn, err := muxado.Dial("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	stream, err := conn.Open()
	if err != nil {
		t.Fatal(err)
	}

	cl := newclient(stream)
	instr := testString("hello, world!")
	var outstr testString
	err = cl.Call("any", &instr, &outstr)
	stream.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkMuxedEcho(b *testing.B) {
	l, err := muxado.Listen("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()
	go serveMuxListener(l, EchoHandler{})

	// wait for listener to start up
	time.Sleep(5 * time.Microsecond)

	conn, err := muxado.Dial("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		stream, err := conn.Open()
		if err != nil {
			b.Fatal(err)
		}
		cl := newclient(stream)
		instr := testString("hello, world!")
		var outstr testString

		for pb.Next() {
			err = cl.Call("any", &instr, &outstr)
			if err != nil {
				b.Fatal(err)
			}
		}
		stream.Close()
	})
	b.StopTimer() // expensive defers
}

func BenchmarkEcho(b *testing.B) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	go serveListener(l, EchoHandler{})
	defer func() {
		l.(*net.TCPListener).SetDeadline(time.Now())
		err := l.Close()
		if err != nil {
			b.Error(err)
		}

		// we have to wait for the listener
		// to *actually* close (b/c net doesn't set SO_REUSEADDR)
		time.Sleep(500 * time.Millisecond)
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

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := cl.Call("any", &instr, &outstr)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer() // b/c we run expensive defers
}
