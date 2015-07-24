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

const Double synapse.Method = 0

type doubleHandler struct{}

func (d doubleHandler) ServeCall(req synapse.Request, res synapse.ResponseWriter) {
	var n Num
	err := req.Decode(&n)
	if err != nil {
		res.Error(synapse.StatusBadRequest, err.Error())
		return
	}
	n.Value *= 2
	res.Send(&n)
}

func main() {
	rt := synapse.RouteTable{Double: doubleHandler{}}
	l, err := net.Listen("tcp", "localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("listening on :7070...")
	fmt.Println(synapse.Serve(l, &rt))
}
