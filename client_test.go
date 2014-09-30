package synapse

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()

	go Serve(l, EchoHandler{})

	cl, err := Dial("localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	defer cl.ForceClose()

	const concurrent = 5
	wg := new(sync.WaitGroup)
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			instr := testString("hello, world!")
			var outstr testString
			err = cl.Call("any", &instr, &outstr)
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
}

func BenchmarkEcho(b *testing.B) {
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()
	go Serve(l, EchoHandler{})
	cl, err := Dial("localhost:7000")
	if err != nil {
		b.Fatal(err)
	}
	defer cl.ForceClose()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(5)
	b.RunParallel(func(pb *testing.PB) {
		instr := testString("hello, world!")
		var outstr testString
		for pb.Next() {
			err = cl.Call("any", &instr, &outstr)
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
