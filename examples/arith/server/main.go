package main

import (
	"fmt"
	"github.com/philhofer/synapse"
	"net"
)

type DoubleHandler struct{}

func (d DoubleHandler) ServeCall(req synapse.Request, res synapse.ResponseWriter) {
	var in Num
	err := req.Decode(&in)
	if err != nil {
		res.WriteHeader(synapse.ServerError)
		return
	}

	var out Num
	out.Value = 2 * in.Value
	res.Send(&out)
}

func main() {
	mux := synapse.NewMux()
	mux.Register("double", DoubleHandler{})

	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(synapse.Serve(l, mux))
}
