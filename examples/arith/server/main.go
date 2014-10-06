package main

import (
	"fmt"
	"github.com/philhofer/synapse"
	"net"
)

func main() {
	mux := synapse.NewRouter()
	mux.HandleFunc("double", func(req synapse.Request, res synapse.ResponseWriter) {
		var in Num
		err := req.Decode(&in)
		if err != nil {
			res.Error(synapse.BadRequest)
			return
		}
		var out Num
		out.Value = 2 * in.Value
		res.Send(&out)
	})

	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(synapse.Serve(l, mux))
}
