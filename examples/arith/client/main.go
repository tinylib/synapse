package main

import (
	"fmt"
	"github.com/philhofer/synapse"
)

func main() {
	cl, err := synapse.Dial("localhost:7000")
	if err != nil {
		fmt.Println(err)
		return
	}

	pi := 3.14159
	num := Num{Value: pi}

	var out Num
	err = cl.Call("double", &num, &out)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Pi * 2 is", out.Value)
}
