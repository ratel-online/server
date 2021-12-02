package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
)

type waiting struct{}

func (s *waiting) Next(player *model.Player) (consts.StateID, error) {
	err := player.WriteString("You joined room!\n")
	if err != nil {
		return 0, player.WriteError(err)
	}
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	access := false
	for {
		signal, err := player.AskForString(player.Terminal())
		if err != nil {
			return 0, player.WriteError(err)
		}
		if signal == "exit" || signal == "e" {
			break
		}
		if signal == "start" || signal == "s" {
			access = true
			break
		}
	}
	if access {
		err = player.WriteString("Game starting!")
		if err != nil {
			return 0, player.WriteError(err)
		}
		if room.Type == consts.GameTypeClassic {
			return consts.StateClassics, nil
		}
	}
	return s.Back(player), nil
}

func (*waiting) Back(player *model.Player) consts.StateID {
	roomId := player.RoomID
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(roomId, fmt.Sprintf("\r\r%s exited room!\n", player.Name))
	return consts.StateHome
}
