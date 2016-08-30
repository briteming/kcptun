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

	laddr := "127.0.0.1:12948"
	if len(os.Args) > 1 {
		laddr = os.Args[1]
	}

	if laddr == "-h" || len(os.Args) > 2 {
		fmt.Printf("Usage: ./socks5 127.0.0.1:12948\n")
		return
	}

	// Create SOCKS5 proxy
	if err := server.ListenAndServe("tcp", laddr); err != nil {
		panic(err)
	}
}
