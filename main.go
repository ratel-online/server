package main

import (
	"fmt"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/network"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("main", err)
			async.PrintStackTrace(err)
		}
	}()
	server := network.NewTcpServer(":9999")
	log.Error(server.Serve())
}
