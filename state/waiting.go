package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
	"time"
)

type waiting struct{}

func (s *waiting) Next(player *model.Player) (consts.StateID, error) {
	err := player.WriteString("You joined room!\n")
	if err != nil {
		return 0, player.WriteError(err)
	}
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
		if room.Creator == player.ID && (signal == "start" || signal == "s") {
			access = true
			room.State = consts.RoomStateRunning
			room.Game, err = initGame(room)
			if err != nil {
				return 0, err
			}
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
	roomId := player.RoomID
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(roomId, fmt.Sprintf("%s exited room!\n", player.Name))
	return consts.StateHome
}

func initGame(room *model.Room) (*model.Game, error) {
	if room.Type == consts.GameTypeClassic {
		return initClassicsGame(room)
	}
	return nil, nil
}
