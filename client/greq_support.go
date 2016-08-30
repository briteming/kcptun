// +build windows darwin freebsd

package main

import (
	"errors"
)

func GetMreq(fd int) ([]byte, error) {
	return nil, errors.New("not support mreq")
}
