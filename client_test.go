package synapse

import (
	"net"
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

	instr := testString("hello, world!")
	var outstr testString

	for i := 0; i < 10; i++ {
		err = cl.Call("any", &instr, &outstr)
		if err != nil {
			t.Fatal(err)
		}
		if instr != outstr {
			t.Fatalf("%q in; %q out", instr, outstr)
		}
	}

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
