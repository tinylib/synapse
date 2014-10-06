package main

import (
	"bytes"
	"fmt"
	"github.com/philhofer/synapse"
)

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

	var buf bytes.Buffer
	out := synapse.JSPipe(&buf)

	// the call to Read blocks
	// until we get a response
	err = res.Read(out)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(buf.String())
}
