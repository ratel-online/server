package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
	"time"
)

type waiting struct{}

func (*waiting) Init(player *model.Player) error {
	return player.WriteString("You joined room!\n")
}

func (*waiting) Next(player *model.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	for {
		time.Sleep(500 * time.Millisecond)
		if database.GetRoomPlayers(room.ID) == room.Players {
			break
		}
	}
	err := player.WriteString("Game starting!")
	if err != nil {
		return 0, player.WriteError(err)
	}
	if room.Type == consts.GameTypeClassic {
		return consts.StateClassics, nil
	}
	return 0, consts.ErrorsExist
}

func (*waiting) Back(player *model.Player) consts.StateID {
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s exited room!\n", player.Name))
	return consts.StateHome
}
