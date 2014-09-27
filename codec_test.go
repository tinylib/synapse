package synapse

import (
	"net"
	"net/rpc"
	"testing"
)

func TestCodecs(t *testing.T) {
	// create fake network
	cl, srv := net.Pipe()

	ccd := &clientCodec{conn: cl}
	scd := &serverCodec{conn: srv}

	server := rpc.NewServer()
	server.Register(Echo{})
	go server.ServeCodec(scd)

	client := rpc.NewClientWithCodec(ccd)

	out := &Arg2{}
	in := &Arg1{Value: "Hello, world!"}
	err := client.Call("Echo.Echo", in, out)
	if err != nil {
		t.Fatal(err)
	}

	if out.Value != in.Value {
		t.Fatalf("%q in; %q out", in.Value, out.Value)
	}

	cl.Close()
}

func BenchmarkBasicCodec(b *testing.B) {
	cl, srv := net.Pipe()
	ccd := &clientCodec{conn: cl}
	scd := &serverCodec{conn: srv}
	server := rpc.NewServer()
	server.Register(Echo{})
	go server.ServeCodec(scd)
	client := rpc.NewClientWithCodec(ccd)
	out := &Arg2{}
	in := &Arg1{Value: "Hello, world!"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := client.Call("Echo.Echo", in, out)
		if err != nil {
			b.Fatal(err)
		}
	}
	cl.Close()
}
