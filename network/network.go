package network

import (
	"fmt"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state"
)

// Network is interface of all kinds of network.
type Network interface {
	Serve() error
}

func handle(rwc protocol.ReadWriteCloser) {
	c := network.Wrapper(rwc)
	defer func() {
		err := c.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	authInfo, err := loginAuth(c)
	if err != nil || authInfo.ID == 0 {
		_ = c.Write(protocol.ErrorPacket(consts.ErrorsAuthFail))
		return
	}

	player := database.RegisterPlayer(c, authInfo)
	player.State(state.Root())

	err = c.Accept(func(packet protocol.Packet, c *network.Conn) {
		fmt.Println(string(packet.Body))
	})
	if err != nil {
		log.Error(err)
	}
}

func loginAuth(c *network.Conn) (*model.AuthInfo, error) {
	packet, err := c.Read()
	if err != nil {
		return nil, err
	}
	authInfo := model.AuthInfo{}
	err = packet.Unmarshal(&authInfo)
	if err != nil {
		return nil, err
	}
	return &authInfo, nil
}
