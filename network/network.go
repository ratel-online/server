package network

import (
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/service"
)

// Network is interface of all kinds of network.
type Network interface {
	Serve() error
}

func handle(rwc protocol.ReadWriteCloser) error {
	player := service.NewPlayer(network.Wrapper(rwc))
	defer player.Offline()
	if player.Auth() != nil {
		return consts.ErrorsAuthFail
	}
	return player.Listening()
}
