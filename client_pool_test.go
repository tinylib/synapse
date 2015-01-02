package synapse

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

func TestClientPool(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()
	mux := NewRouter()
	mux.Handle("echo", EchoHandler{})

	go Serve(l, mux)

	cl, err := DialCluster("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	err = cl.Add("localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	const concurrent = 5
	wg := new(sync.WaitGroup)
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			instr := testData("hello, world!")
			var outstr testData
			err := cl.Call("echo", &instr, &outstr)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(instr), []byte(outstr)) {
				t.Fatalf("%q in; %q out", instr, outstr)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	// test for NotFound
	res, err := cl.Async("doesn't-exist", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = res.Read(nil)
	if err != NotFound {
		t.Errorf("got error %q; expected %q", err, NotFound)
	}
}

func BenchmarkClientPool(b *testing.B) {
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
	cl, err := DialCluster("tcp", "localhost:7000", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer cl.Close()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(10)
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

func TestClientPoolStats(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()
	mux := NewRouter()
	mux.Handle("echo", EchoHandler{})

	go Serve(l, mux)

	// intentionally dial one bad host
	cl, err := DialCluster("tcp", "localhost:7000", "localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	stats := cl.Status()

	if len(stats.Connected) != 1 {
		t.Errorf("expected 1 connection; found %d", len(stats.Connected))
	}

	if len(stats.Disconnected) != 1 {
		t.Errorf("expected 1 bad connection; found %d", len(stats.Disconnected))
	}

	// now we just carry out the
	// same test as before; the
	// pool should have ejected
	// the bad connection, so we
	// shouldn't observe any errors

	const concurrent = 5
	wg := new(sync.WaitGroup)
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			instr := testData("hello, world!")
			var outstr testData
			err := cl.Call("echo", &instr, &outstr)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(instr), []byte(outstr)) {
				t.Fatalf("%q in; %q out", instr, outstr)
			}
			wg.Done()
		}()
	}

	wg.Wait()

}
