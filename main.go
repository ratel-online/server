package main

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/network"
)

func main() {
	server := network.NewTcpServer(":9999")
	log.Error(server.Serve())
}
