package network

import (
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state"
)

// Network is interface of all kinds of network.
type Network interface {
	Serve() error
}

func handle(rwc protocol.ReadWriteCloser) error {
	// 给新进入的用户分配资源
	c := network.Wrapper(rwc)
	defer func() {
		err := c.Close()
		if err != nil {
			log.Error(err)
		}
	}()
	log.Info("new player connected! ")
	authInfo, err := loginAuth(c)
	if err != nil {
		_ = c.Write(protocol.ErrorPacket(err))
		return err
	}
	player, cs, tookOver, err := database.Connected(c, authInfo)
	if err != nil {
		_ = c.Write(protocol.ErrorPacket(err))
		return err
	}
	log.Infof("player auth accessed, ip %s, %d:%s, takeover: %v\n", player.IP, player.ID, player.Name, tookOver)
	initial := consts.StateWelcome
	if tookOver {
		initial = database.ResumeState(player)
	}
	go state.RunFrom(player, initial)
	// 用本次连接的那一代去关闭：被顶号时旧协程不会误关新连接，也不会清掉房间
	defer player.OfflineConn(cs)
	return player.Listening(cs)
}

// 登陆验签
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
