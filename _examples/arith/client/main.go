package main

import (
	"fmt"
	"github.com/philhofer/synapse"
	"os"
)

//go:generate msgp -io=false

type Num struct {
	Value float64 `msg:"val"`
}

func main() {
	cl, err := synapse.DialTCP("localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer cl.Close()

	pi := 3.14159
	num := Num{Value: pi}

	// here we make an asynchronous
	// call to the service
	fmt.Println("Asking the remote server to double 3.14159 for us...")
	res, err := cl.Async("double", &num)
	if err != nil {
		fmt.Println(err)
		return
	}

	// the call to Read blocks
	// until we get a response.
	// JSPipe sends whatever data
	// is returned to an io.Writer as
	// JSON.
	err = res.Read(synapse.JSPipe(os.Stdout))
	if err != nil {
		fmt.Println(err)
	}
}
