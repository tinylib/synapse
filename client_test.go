package synapse

import (
	"github.com/inconshreveable/muxado"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	l, err := muxado.Listen("tcp", "localhost:7000")
	if err != nil {
		t.Fatal(err)
	}

	go serveMuxListener(l, EchoHandler{})
	defer func() {
		l.Close()
		time.Sleep(1 * time.Millisecond)
	}()

	cl, err := Dial("localhost:7000")
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	instr := testString("hello, world!")
	var outstr testString

	for i := 0; i < 5; i++ {
		err = cl.Call("any", &instr, &outstr)
		if err != nil {
			t.Fatal(err)
		}
	}

}
