package main

import (
	"fmt"
	"github.com/tinylib/synapse"
	"os"
	"time"
)

//go:generate msgp -io=false

type Num struct {
	Value float64 `msg:"val"`
}

const Double synapse.Method = 0

func main() {
	cl, err := synapse.Dial("tcp", "localhost:7000", time.Second)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cl.Close()

	pi := 3.14159
	num := Num{Value: pi}

	fmt.Println("Asking the remote server to double 3.14159 for us...")
	err = cl.Call(Double, &num, synapse.JSPipe(os.Stdout))
	if err != nil {
		fmt.Println("ERROR:", err)
	}
}
