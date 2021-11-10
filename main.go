package main

import (
    "github.com/ratel-online/server/network"
    "log"
)

func main() {
    server := network.NewTcpServer(":5555")
    log.Fatalln(server.Serve())
}