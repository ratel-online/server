package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
)

type join struct{}

func (*join) Init(player *model.Player) error {
	buf := bytes.Buffer{}
	rooms := database.GetRooms()
	buf.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", "ID", "Type", "Players", "State"))
	for _, room := range rooms {
		buf.WriteString(fmt.Sprintf("%d\t%s\t%d\t%s\n", room.ID, consts.GameTypes[room.Type], database.GetRoomPlayers(player.RoomID), consts.RoomStates[room.State]))
	}
	buf.Truncate(buf.Len() - 1)
	return player.WriteString(buf.String())
}

func (*join) Next(player *model.Player) (consts.StateID, error) {
	roomId, err := player.AskForInt64()
	if err != nil {
		return 0, player.WriteError(err)
	}
	err = database.JoinRoom(roomId, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	err = database.RoomBroadcast(roomId, fmt.Sprintf("%s joined room!", player.Name), player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*join) Back(player *model.Player) consts.StateID {
	return consts.StateHome
}
