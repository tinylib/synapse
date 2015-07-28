package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/tinylib/msgp/msgp"
	"github.com/tinylib/synapse"
)

var (
	sock = flag.String("sock", "ssvc_sock", "unix socket to listen on")
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

type echoHandler struct{}

func (e echoHandler) ServeCall(req synapse.Request, res synapse.ResponseWriter) {
	var r msgp.Raw
	if err := req.Decode(&r); err != nil {
		perror("server: error on Request.Decode():", err)
	}
	if err := res.Send(r); err != nil {
		perror("server: error on ResponseWriter.Send():", err)
	}

	sv := synapse.Services("service-test")
	for _, s := range sv {
		fmt.Println("serrver: known service:", s)
	}
}

func init() {
	synapse.ErrorLogger = log.New(os.Stderr, "syn-server-log: ", log.LstdFlags)
}

func main() {
	go func() {
		err := synapse.ListenAndServe("tcp", *port, "service-test", echoHandler{})
		if err != nil {
			perror("server: listen tcp error:", err)
		}
	}()
	ul, err := net.Listen("unix", *sock)
	if err != nil {
		perror("server: listen unix error:", err)
	}

	// cleanup unix socket
	// on kill so we don't
	// leave it lying around
	go func() {
		in := make(chan os.Signal, 1)
		signal.Notify(in, os.Interrupt)
		<-in
		ul.Close()
		fmt.Println("server: exiting successfully")
		os.Exit(0)
	}()

	err = synapse.Serve(ul, "service-test", echoHandler{})
	if err != nil && !strings.Contains(err.Error(), "closed") {
		perror("server: serve unix error:", err)
	}
}
