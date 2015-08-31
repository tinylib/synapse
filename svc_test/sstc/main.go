package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tinylib/synapse"
)

var (
	port = flag.String("port", ":7005", "tcp port to dial")
)

func fatalln(str string) {
	fmt.Println(str)
	os.Exit(1)
}

func perror(str string, err error) {
	fmt.Println(str, err)
	os.Exit(1)
}

func dumpservices(srv string) {
	fmt.Print("Known addresses for service ", srv)
	sv := synapse.Services(srv)
	if len(sv) == 0 {
		fmt.Print(": None.\n")
		return
	}
	fmt.Print("\n")
	for _, s := range sv {
		fmt.Println("\t", s)
	}
}

func init() {
	synapse.ErrorLogger = log.New(os.Stderr, "syn-client-log: ", log.LstdFlags)
}

func main() {
	cl, err := synapse.Dial("tcp", *port, 25*time.Millisecond)
	if err != nil {
		perror("client: dial failure:", err)
	}
	fmt.Println("client: connected to service", cl.Service())

	err = cl.Call(0, synapse.String("hello!"), nil)
	if err != nil {
		perror("client: call error:", err)
	}

	// wait for service lists to synchronize
	time.Sleep(50 * time.Millisecond)

	dumpservices(cl.Service())

	ss := synapse.Nearest(cl.Service())
	if ss == nil {
		fatalln("client: Nearest() returned nil")
	}

	nwk, sock := ss.Addr()
	if nwk != "unix" {
		fatalln("client: Nearest(service).Addr() didn't return a unix socket")
	}

	fmt.Println("client: found socket", sock)
	var cl2 *synapse.Client

	cl2, err = synapse.Dial(nwk, sock, 25*time.Millisecond)
	if err != nil {
		perror("client: couldn't dial socket:", err)
	}
	// give the second client time
	// to complete another round of
	// list synchronization.
	time.Sleep(50 * time.Millisecond)
	err = cl2.Close()
	if err != nil {
		perror("client: close error:", err)
	}

	err = cl.Close()
	if err != nil {
		perror("client: close error:", err)
	}
	fmt.Println("client: exiting successfully")
}
