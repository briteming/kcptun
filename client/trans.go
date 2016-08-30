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
		Timeout:   6 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	listener net.Listener
)

func serverTrans(addrStr string, proxyAddr string) error {
	u, err := url.Parse("socks5://" + proxyAddr)
	if err != nil {
		log.Println(err)
		return err
	}

	proxy_dialer, err := proxy.FromURL(u, defaultDialer)
	if err != nil {
		log.Println(err)
		return err
	}

	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	checkError(err)
	l, err := net.ListenTCP("tcp", addr)
	checkError(err)
	log.Println("listening redir on:", l.Addr())

	for {
		tcp_conn, err := l.AcceptTCP()
		if err != nil {
			log.Println(err)
			return err
		}
		go handleTransConnection(tcp_conn, proxy_dialer)
	}
}

func handleTransConnection(c *net.TCPConn, dialer proxy.Dialer) {
	isClosed := false
	defer func() {
		if !isClosed {
			c.Close()
		}
	}()

	addr, err := getOriginalDst(c)
	if err != nil {
		log.Printf("original err: %v\n", err)
		return
	}

	pconn, err := dialer.Dial("tcp", addr)
	if err != nil {
		log.Printf("dial err: %v\n", err)
		return
	}
	defer func() {
		if !isClosed {
			pconn.Close()
		}
	}()

	isClosed = true
	handleClient(c, pconn)
}

func getOriginalDst(conn *net.TCPConn) (string, error) {
	f, err := conn.File()
	if err != nil {
		return "", err
	}
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
