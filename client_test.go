package synapse

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

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
			err := tcpClient.Call("echo", &instr, &outstr)
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
	res, err := tcpClient.Async("doesn't-exist", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = res.Read(nil)
	if err != NotFound {
		t.Errorf("got error %q; expected %q", err, NotFound)
	}
}

// the output of the debug handler
// is only visible if '-v' is set
func TestDebugClient(t *testing.T) {
	attachDebug("debug_echo", t)

	instr := String("here's a message body!")
	var outstr String
	err := tcpClient.Call("debug_echo", &instr, &outstr)
	if err != nil {
		t.Fatal(err)
	}
	if instr != outstr {
		t.Fatal("input and output not equal")
	}
}

func TestAsyncClient(t *testing.T) {

	// make 5 requests, then
	// read 5 responses
	const concurrent = 5
	var hlrs [concurrent]AsyncResponse
	instr := testData("hello, world!")
	for i := 0; i < concurrent; i++ {
		var err error
		hlrs[i], err = tcpClient.Async("echo", &instr)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := range hlrs {
		var outstr testData
		err := hlrs[i].Read(&outstr)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal([]byte(instr), []byte(outstr)) {
			t.Errorf("%q in; %q out", instr, outstr)
		}
	}

}

// test that 'nil' is a safe
// argument to requests and responses
func TestNop(t *testing.T) {
	err := tcpClient.Call("nop", nil, nil)
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
	mux := NewRouter()
	mux.Handle("echo", EchoHandler{})

	go Serve(l, mux)
	cl, err := Dial("tcp", "localhost:7000", 1)
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
			err = cl.Call("echo", &instr, &outstr)
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

// this is basically the fastest possible
// arrangement; we have a no-op (no body)
// request and response, and we're using
// unix sockets.
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
	cl, err := Dial("unix", "bench", 5)
	if err != nil {
		b.Fatal(err)
	}
	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := cl.Call("any", nil, nil)
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

	cl, err := NewClient(cln, 100)
	if err != nil {
		b.Fatal(err)
	}

	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := cl.Call("any", nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.StopTimer()
}
