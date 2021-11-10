package network

import (
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"log"
)

// Network is interface of all kinds of network.
type Network interface {
	Serve() error
}

func handle(rwc protocol.ReadWriteCloser) {
	c := network.Wrapper(rwc)
	err := c.Accept(func(packet protocol.Packet, c *network.Conn) {

	})
	if err != nil {
		log.Printf("c.Accept err %v\n", err)
	}
}
