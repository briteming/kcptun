package main

import (
	"encoding/json"
	"os"
	"strconv"
)

type Config struct {
	LocalAddr    string `json:"localaddr"`
	RemoteAddr   string `json:"remoteaddr"`
	Key          string `json:"key"`
	Crypt        string `json:"crypt"`
	Mode         string `json:"mode"`
	Conn         int    `json:"conn"`
	AutoExpire   int    `json:"autoexpire"`
	MTU          int    `json:"mtu"`
	SndWnd       int    `json:"sndwnd"`
	RcvWnd       int    `json:"rcvwnd"`
	DataShard    int    `json:"datashard"`
	ParityShard  int    `json:"parityshard"`
	DSCP         int    `json:"dscp"`
	NoComp       bool   `json:"nocomp"`
	AckNodelay   bool   `json:"acknodelay"`
	NoDelay      int    `json:"nodelay"`
	Interval     int    `json:"interval"`
	Resend       int    `json:"resend"`
	NoCongestion int    `json:"nc"`
	SockBuf      int    `json:"sockbuf"`
	KeepAlive    int    `json:"keepalive"`
	Redir        string `json:"redir"`

	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	Password   string `json:"password"`
	RedirPort  int    `json:"redir_port"`
	Socks5Port int    `json:"socks5_port"`
}

func parseJsonConfig(config *Config, path string) error {
	file, err := os.Open(path) // For read access.
	if err != nil {
		return err
	}
	defer file.Close()

	if err = json.NewDecoder(file).Decode(config); err != nil {
		return err
	}

	if config.LocalAddr == "" || config.LocalAddr == ":12948" {
		config.LocalAddr = ":" + strconv.Itoa(config.Socks5Port)
	}
	if config.Key == "" || config.Key == "it's a secrect" {
		config.Key = config.Password
	}
	if config.RemoteAddr == "" || config.RemoteAddr == "vps:29900" {
		config.RemoteAddr = config.Server + ":" + strconv.Itoa(config.ServerPort)
	}
	if config.Redir == "" && config.RedirPort > 0 {
		config.Redir = ":" + strconv.Itoa(config.RedirPort)
	}

	return err
}
