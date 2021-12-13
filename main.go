package main

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/network"
)

func main() {
	async.Async(func() {
		wsServer := network.NewWebsocketServer(":9998")
		log.Panic(wsServer.Serve())
	})

	server := network.NewTcpServer(":9999")
	log.Panic(server.Serve())
}
