package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
	"strings"
	"time"
)

type waiting struct{}

func (s *waiting) Next(player *model.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, consts.ErrorsExist
	}
	access := false
	for {
		signal, err := player.AskForString(time.Second)
		if err != nil && err != consts.ErrorsTimeout {
			return 0, err
		}
		if room.State == consts.RoomStateRunning {
			access = true
			break
		}
		signal = strings.ToLower(signal)
		if room.Creator == player.ID && room.Players > 1 && (signal == "start" || signal == "s") {
			access = true
			room.Game, err = initGame(room)
			if err != nil {
				return 0, err
			}
			room.State = consts.RoomStateRunning
			break
		}
	}
	if access {
		if room.Type == consts.GameTypeClassic {
			return consts.StateClassics, nil
		}
	}
	return s.Exit(player), nil
}

func (*waiting) Exit(player *model.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		database.LeaveRoom(player.RoomID, player.ID)
		database.RoomBroadcast(room.ID, fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players))
	}
	return consts.StateHome
}

func initGame(room *model.Room) (*model.Game, error) {
	if room.Type == consts.GameTypeClassic {
		return initClassicsGame(room)
	}
	return nil, nil
}
