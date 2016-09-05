package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/proxy"
)

const (
	ReadTimeout     = (10 * time.Second)
	SocksVer5       = 5
	SocksCmdConnect = 1
)

var (
	errAddrType      = errors.New("socks addr type not supported")
	errVer           = errors.New("socks version not supported")
	errMethod        = errors.New("socks only support 1 method now")
	errAuthExtraData = errors.New("socks authentication get extra data")
	errReqExtraData  = errors.New("socks request get extra data")
	errCmd           = errors.New("socks command not supported")
	errConnection    = errors.New("error connection")

	defaultDialer = &net.Dialer{
		Timeout:   6 * time.Second,
		KeepAlive: 60 * time.Second,
	}
)

func handleSocks5Client(p1, p2 io.ReadWriteCloser) (err error) {
	//p1 tcpconn, p2 mux conn
	closed := false
	var addr string

	defer func() {
		log.Println("closed addr", addr)
		if !closed {
			p1.Close()
			p2.Close()
		}
	}()

	obuf := make([]byte, 304)
	if err = handShake(obuf, p1); err != nil {
		log.Println("socks handshake:", err)
		return
	}
	rawaddr, addr, err := getRequest(obuf, p1)
	log.Println("new socks5 conn by", addr)
	if err != nil {
		log.Println("error getting request:", err)
		return
	}
	// Sending connection established message immediately to client.
	// This some round trip time for creating socks connection with the client.
	// But if connection failed, the client will get connection reset error.
	_, err = p1.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
	if err != nil {
		log.Println("send connection confirmation", err)
		return
	}

	p2.Write(rawaddr)

	closed = true
	handleClient(p1, p2)
	return
}

func handShake(obuf []byte, conn io.ReadWriteCloser) (err error) {
	const (
		idVer     = 0
		idNmethod = 1
	)
	// version identification and method selection message in theory can have
	// at most 256 methods, plus version and method field in total 258 bytes
	// the current rfc defines only 3 authentication methods (plus 2 reserved),
	// so it won't be such long in practice

	buf := obuf[:258]

	var n int
	//conn.SetReadDeadline(time.Now().Add(ReadTimeout))

	// make sure we get the nmethod field
	if n, err = io.ReadAtLeast(conn, buf, idNmethod+1); err != nil {
		return
	}
	if buf[idVer] != SocksVer5 {
		return errVer
	}
	nmethod := int(buf[idNmethod])
	msgLen := nmethod + 2
	if n == msgLen { // handshake done, common case
		// do nothing, jump directly to send confirmation
	} else if n < msgLen { // has more methods to read, rare case
		if _, err = io.ReadFull(conn, buf[n:msgLen]); err != nil {
			return
		}
	} else { // error, should not get extra data
		return errAuthExtraData
	}
	// send confirmation: version 5, no authentication required
	_, err = conn.Write([]byte{SocksVer5, 0})
	return
}

func getRequest(obuf []byte, conn io.ReadWriteCloser) (rawaddr []byte, host string, err error) {
	const (
		idVer   = 0
		idCmd   = 1
		idType  = 3 // address type index
		idIP0   = 4 // ip addres start index
		idDmLen = 4 // domain address length index
		idDm0   = 5 // domain address start index

		typeIPv4 = 1 // type is ipv4 address
		typeDm   = 3 // type is domain address
		typeIPv6 = 4 // type is ipv6 address

		lenIPv4   = 3 + 1 + net.IPv4len + 2 // 3(ver+cmd+rsv) + 1addrType + ipv4 + 2port
		lenIPv6   = 3 + 1 + net.IPv6len + 2 // 3(ver+cmd+rsv) + 1addrType + ipv6 + 2port
		lenDmBase = 3 + 1 + 1 + 2           // 3 + 1addrType + 1addrLen + 2port, plus addrLen
	)
	// refer to getRequest in server.go for why set buffer size to 263
	buf := obuf[:263]

	var n int
	//conn.SetReadDeadline(time.Now().Add(ReadTimeout))
	// read till we get possible domain length field
	if n, err = io.ReadAtLeast(conn, buf, idDmLen+1); err != nil {
		return
	}
	// check version and cmd
	if buf[idVer] != SocksVer5 {
		err = errors.New("version error")
		return
	}
	if buf[idCmd] != SocksCmdConnect {
		err = errCmd
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
		err = errAddrType
		return
	}

	if n == reqLen {
		// common case, do nothing
	} else if n < reqLen { // rare case
		if _, err = io.ReadFull(conn, buf[n:reqLen]); err != nil {
			return
		}
	} else {
		err = errReqExtraData
		return
	}

	rawaddr = buf[idType:reqLen]
	//log.Println("n=", n, reqLen)

	switch buf[idType] {
	case typeIPv4:
		host = net.IP(buf[idIP0 : idIP0+net.IPv4len]).String()
	case typeIPv6:
		host = net.IP(buf[idIP0 : idIP0+net.IPv6len]).String()
	case typeDm:
		host = string(buf[idDm0 : idDm0+buf[idDmLen]])
	}
	port := binary.BigEndian.Uint16(buf[reqLen-2 : reqLen])
	host = net.JoinHostPort(host, strconv.Itoa(int(port)))

	//conn.SetReadDeadline(time.Time{})
	return
}

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
