package texas

import (
	"time"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

var (
	stateBet     = 1
	stateWaiting = 2
)

type Texas struct{}

func (g *Texas) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.Texas)

	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		texasPlayer := game.Player(player.ID)
		if texasPlayer == nil {
			return 0, player.WriteError(consts.ErrorsExist)
		}
		select {
		case state, ok := <-texasPlayer.State:
			if !ok {
				return 0, consts.ErrorsChanClosed
			}
			switch state {
			case stateBet:
				err := bet(player, game)
				if err != nil {
					log.Error(err)
					return 0, err
				}
			case stateWaiting:
				return consts.StateWaiting, nil
			default:
				return 0, consts.ErrorsChanClosed
			}
		case <-time.After(5 * time.Second):
			// 防止通道阻塞导致的死锁
			return 0, consts.ErrorsTimeout
		}
	}
}

func (*Texas) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}
