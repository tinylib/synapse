package synapse

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

func isCode(err error, c Status) bool {
	if reserr, ok := err.(*ResponseError); ok && reserr.Code == c {
		return true
	}
	return false
}

// open up a client and server; make
// some concurrent requests
func TestClient(t *testing.T) {

	const concurrent = 5
	wg := new(sync.WaitGroup)
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			instr := testData("hello, world!")
			var outstr testData
			err := tcpClient.Call(Echo, &instr, &outstr)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(instr), []byte(outstr)) {
				t.Fatal("input and output not equal")
			}
			wg.Done()
		}()
	}

	wg.Wait()

	// test for NotFound
	err := tcpClient.Call(100, nil, nil)
	if !isCode(err, StatusNotFound) {
		t.Errorf("expected not-found error; got %#v", err)
	}
}

// the output of the debug handler
// is only visible if '-v' is set
func TestDebugClient(t *testing.T) {
	instr := String("here's a message body!")
	var outstr String
	err := tcpClient.Call(DebugEcho, &instr, &outstr)
	if err != nil {
		t.Fatal(err)
	}
	if instr != outstr {
		t.Fatal("input and output not equal")
	}
}

// test that 'nil' is a safe
// argument to requests and responses
func TestNop(t *testing.T) {
	err := tcpClient.Call(Nop, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// benchmarks the test case above
func BenchmarkTCPEcho(b *testing.B) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()

	go Serve(l, EchoHandler{})
	cl, err := Dial("tcp", "localhost:7000", 50*time.Millisecond)
	if err != nil {
		b.Fatal(err)
	}
	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		instr := testData("hello, world!")
		var outstr testData
		for pb.Next() {
			err := cl.Call(Echo, &instr, &outstr)
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal([]byte(instr), []byte(outstr)) {
				b.Fatalf("%q in; %q out", instr, outstr)
			}
		}
	})
	b.StopTimer()
}

func BenchmarkUnixNoop(b *testing.B) {
	l, err := net.Listen("unix", "bench")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()
	go Serve(l, NopHandler{})
	cl, err := Dial("unix", "bench", 50*time.Millisecond)
	if err != nil {
		b.Fatal(err)
	}
	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := cl.Call(Echo, nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
}

func BenchmarkPipeNoop(b *testing.B) {
	srv, cln := net.Pipe()

	go ServeConn(srv, NopHandler{})

	defer srv.Close()

	cl, err := NewClient(cln, 50*time.Millisecond)
	if err != nil {
		b.Fatal(err)
	}

	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := cl.Call(0, nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
}
