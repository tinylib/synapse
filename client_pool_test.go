package synapse

import (
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

	err = cl.Add("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	const concurrent = 5
	wg := new(sync.WaitGroup)
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			instr := testString("hello, world!")
			var outstr testString
			err := cl.Call("echo", &instr, &outstr)
			if err != nil {
				t.Fatal(err)
			}
			if instr != outstr {
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
		instr := testString("hello, world!")
		var outstr testString
		for pb.Next() {
			err = cl.Call("echo", &instr, &outstr)
			if err != nil {
				b.Fatal(err)
			}
			if instr != outstr {
				b.Fatalf("%q in; %q out", instr, outstr)
			}
		}
	})
	b.StopTimer()
}
