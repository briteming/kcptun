// +build linux

package main

import (
	"syscall"
)

const (
	SO_ORIGINAL_DST = 80
)

func GetMreq(fd int) ([16]byte, error) {
	//https://github.com/cybozu-go/transocks/blob/master/original_dst_linux.go#L44
	if err := syscall.SetNonblock(fd, true); err != nil {
		return [16]byte{}, err
	}

	addr, err2 := syscall.GetsockoptIPv6Mreq(fd, syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	if err2 != nil {
		return [16]byte{}, err2
	} else {
		return addr.Multiaddr, nil
	}
}
