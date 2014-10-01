package main

import (
	"fmt"
	"github.com/philhofer/synapse"
)

func main() {
	cl, err := synapse.DialTCP("localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}

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

	// do some stuff...
	fmt.Println("Waiting for a response. Would you like a coffee while you wait? (Y/N)")

	// the call to Read blocks
	// until we get a response
	var out Num
	err = res.Read(&out)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Too late! We already got a response.")

	fmt.Println("Pi * 2 is", out.Value)
}
