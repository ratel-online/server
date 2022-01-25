package main

import (
	"flag"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/network"
	"strconv"
)

var (
	Wsport  int
	Tcpport int
)

func main() {
	flag.IntVar(&Wsport, "w", 9998, "WebsocketServer Port")
	flag.IntVar(&Tcpport, "t", 9999, "TcpServer Port")
	flag.Parse()

	async.Async(func() {
		wsServer := network.NewWebsocketServer(":" + strconv.Itoa(Wsport))
		log.Panic(wsServer.Serve())
	})

	server := network.NewTcpServer(":" + strconv.Itoa(Tcpport))
	log.Panic(server.Serve())
}
