package main

import (
	"fmt"
	"github.com/tinylib/synapse"
	"net"
)

//go:generate msgp -io=false

type Num struct {
	Value float64 `msg:"val"`
}

func main() {
	mux := synapse.NewRouter()
	mux.HandleFunc("double", func(req synapse.Request, res synapse.ResponseWriter) {
		var n Num
		err := req.Decode(&n)
		if err != nil {
			res.Error(synapse.BadRequest)
			return
		}
		n.Value *= 2
		res.Send(&n)
	})

	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(synapse.Serve(l, mux))
}
