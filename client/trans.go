package main

import (
	"encoding/binary"
	"log"
	"net"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

var (
	defaultDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	listener net.Listener
)

func serverTrans(addr string, proxyAddr string) error {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return err
	}

	proxy_dialer, err := proxy.FromURL(u, defaultDialer)
	if err != nil {
		return err
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		tcp_conn, ok := conn.(*net.TCPConn)
		if !ok {
			conn.Close()
			panic("not a TCPConn!")
		}
		go handleTransConnection(tcp_conn, proxy_dialer)
	}
}

func handleTransConnection(c *net.TCPConn, dialer proxy.Dialer) {
	isClosed := false
	func() {
		if !isClosed {
			c.Close()
		}
	}()

	addr, err := getOriginalDst(c)
	if err != nil {
		log.Println(err)
		return
	}

	pconn, err := dialer.Dial("tcp", addr)
	if err != nil {
		log.Println(err)
		return
	}
	func() {
		if !isClosed {
			pconn.Close()
		}
	}()

	isClosed = true
	handleClient(c, pconn)
}

func getOriginalDst(conn *net.TCPConn) (string, error) {
	if f, err := conn.File(); err != nil {
		return "", err
	} else {
		defer f.Close()

		fd := int(f.Fd())

		//TODO for ipv6
		addr, err := GetMreq(fd)
		if err != nil {
			return "", err
		}

		port := binary.BigEndian.Uint16(addr[2:4])
		host := net.JoinHostPort(net.IP(addr[4:8]).String(), strconv.Itoa(int(port)))
		return host, nil
	}
}
