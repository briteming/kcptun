package main

import (
	"fmt"
	"os"

	socks5 "github.com/armon/go-socks5"
)

func main() {
	// Create a SOCKS5 server
	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	laddr := ":9090"
	if len(os.Args) > 1 {
		laddr = os.Args[1]
	}

	if laddr == "-h" || len(os.Args) > 2 {
		fmt.Printf("Usage: ./socks5 0.0.0.0:9090\n")
		return
	}

	// Create SOCKS5 proxy on localhost port 8000
	if err := server.ListenAndServe("tcp", laddr); err != nil {
		panic(err)
	}
}
