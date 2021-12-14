package network

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state"
	"time"
)

// Network is interface of all kinds of network.
type Network interface {
	Serve() error
}

func handle(rwc protocol.ReadWriteCloser) error {
	c := network.Wrapper(rwc)
	defer func() {
		err := c.Close()
		if err != nil {
			log.Error(err)
		}
	}()
	log.Info("new player connected! ")
	authInfo, err := loginAuth(c)
	if err != nil || authInfo.ID == 0 {
		_ = c.Write(protocol.ErrorPacket(consts.ErrorsAuthFail))
		return consts.ErrorsAuthFail
	}
	log.Infof("player auth accessed, %d:%s\n", authInfo.ID, authInfo.Name)
	player := database.Connected(c, authInfo)
	go state.Run(player)
	defer player.Offline()
	return player.Listening()
}

func loginAuth(c *network.Conn) (*model.AuthInfo, error) {
	authChan := make(chan *model.AuthInfo)
	defer close(authChan)
	async.Async(func() {
		packet, err := c.Read()
		if err != nil {
			log.Error(err)
			return
		}
		authInfo := &model.AuthInfo{}
		err = packet.Unmarshal(authInfo)
		if err != nil {
			log.Error(err)
			return
		}
		authChan <- authInfo
	})
	select {
	case authInfo := <-authChan:
		return authInfo, nil
	case <-time.After(3 * time.Second):
		return nil, consts.ErrorsAuthFail
	}
}
