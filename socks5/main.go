package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/armon/go-socks5"
)

const (
	SocksVer5       = 5
	SocksCmdConnect = 1
)

var (
	s5svr *socks5.Server
)

func checkError(err error) {
	if err != nil {
		log.Printf("%+v\n", err)
		os.Exit(-1)
	}
}

type bufferedConn struct {
	r        *bufio.Reader
	net.Conn // So that most methods are embedded
}

func newBufferedConn(c net.Conn) bufferedConn {
	return bufferedConn{bufio.NewReader(c), c}
}

func newBufferedConnSize(c net.Conn, n int) bufferedConn {
	return bufferedConn{bufio.NewReaderSize(c, n), c}
}

func (b bufferedConn) Peek(n int) ([]byte, error) {
	return b.r.Peek(n)
}

func (b bufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func handleClient(p1, p2 io.ReadWriteCloser) {
	log.Println("stream opened")
	defer log.Println("stream closed")
	defer p1.Close()
	defer p2.Close()

	// start tunnel
	p1die := make(chan struct{})
	go func() {
		io.Copy(p1, p2)
		close(p1die)
	}()

	p2die := make(chan struct{})
	go func() {
		io.Copy(p2, p1)
		close(p2die)
	}()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

func socks5ServerSel(conn bufferedConn) error {
	if buf, err := conn.Peek(2); err != nil {
		return err
	} else {
		if buf[0] == SocksVer5 && buf[1] == SocksCmdConnect {
			//normal socks5 server
			return s5svr.ServeConn(conn)
		} else {
			//shadowsocks like socks5 server, reduce latency
			return handleSocks5(conn)
		}
	}
}

func handleSocks5(conn net.Conn) (err error) {
	var host string
	var extra []byte
	var remote net.Conn = nil

	closed := false
	defer func() {
		log.Println("closed", host)
		if !closed {
			conn.Close()
			if remote != nil {
				remote.Close()
			}
		}
	}()

	host, extra, err = getRequest(conn)
	log.Println("connecting to", host)
	if err != nil {
		return
	}

	remote, err = net.Dial("tcp", host)
	if err != nil {
		return
	}

	if extra != nil && len(extra) > 0 {
		if _, err = remote.Write(extra); err != nil {
			log.Println("write request extra error:", err)
			return
		}
	}

	closed = true
	handleClient(remote, conn)
	return
}

func getRequest(conn net.Conn) (host string, extra []byte, err error) {
	const (
		idType  = 0 // address type index
		idIP0   = 1 // ip addres start index
		idDmLen = 1 // domain address length index
		idDm0   = 2 // domain address start index

		typeIPv4 = 1 // type is ipv4 address
		typeDm   = 3 // type is domain address
		typeIPv6 = 4 // type is ipv6 address

		lenIPv4   = 1 + net.IPv4len + 2 // 1addrType + ipv4 + 2port
		lenIPv6   = 1 + net.IPv6len + 2 // 1addrType + ipv6 + 2port
		lenDmBase = 1 + 1 + 2           // 1addrType + 1addrLen + 2port, plus addrLen
	)

	// buf size should at least have the same size with the largest possible
	// request size (when addrType is 3, domain name has at most 256 bytes)
	// 1(addrType) + 1(lenByte) + 256(max length address) + 2(port)
	buf := make([]byte, 260)
	var n int
	// read till we get possible domain length field
	if n, err = io.ReadAtLeast(conn, buf, idDmLen+1); err != nil {
		return
	}

	reqLen := -1
	switch buf[idType] {
	case typeIPv4:
		reqLen = lenIPv4
	case typeIPv6:
		reqLen = lenIPv6
	case typeDm:
		reqLen = int(buf[idDmLen]) + lenDmBase
	default:
		err = errors.New("addr type not supported")
		return
	}

	if n < reqLen { // rare case
		if _, err = io.ReadFull(conn, buf[n:reqLen]); err != nil {
			return
		}
	} else if n > reqLen {
		// it's possible to read more than just the request head
		extra = buf[reqLen:n]
	}

	// Return string for typeIP is not most efficient, but browsers (Chrome,
	// Safari, Firefox) all seems using typeDm exclusively. So this is not a
	// big problem.
	switch buf[idType] {
	case typeIPv4:
		host = net.IP(buf[idIP0 : idIP0+net.IPv4len]).String()
	case typeIPv6:
		host = net.IP(buf[idIP0 : idIP0+net.IPv6len]).String()
	case typeDm:
		host = string(buf[idDm0 : idDm0+buf[idDmLen]])
	}
	// parse port
	port := binary.BigEndian.Uint16(buf[reqLen-2 : reqLen])
	host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	return
}

func main() {
	laddr := ":12948"
	if len(os.Args) > 1 {
		laddr = os.Args[1]
	}

	if laddr == "-h" || len(os.Args) > 2 {
		fmt.Printf("Usage: ./socks5 0.0.0.0:12948\n")
		return
	}

	// Create SOCKS5 proxy
	addr, err := net.ResolveTCPAddr("tcp", laddr)
	checkError(err)
	l, err := net.ListenTCP("tcp", addr)
	checkError(err)
	log.Println("listening socks5 on:", l.Addr())

	conf := &socks5.Config{}
	s5svr, err = socks5.New(conf)

	for {
		tcp_conn, err := l.AcceptTCP()
		if err != nil {
			log.Println(err)
			return
		}
		go socks5ServerSel(newBufferedConn(tcp_conn))
	}
}
