package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
)

type classics struct{}

func (*classics) Next(player *model.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	err := player.WriteString("Game starting!\n")
	if err != nil {
		return 0, player.WriteError(err)
	}
	return 0, nil
}

func (*classics) Back(player *model.Player) consts.StateID {
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s exited room!\n", player.Name))
	return consts.StateHome
}
