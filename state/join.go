package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
	"strconv"
)

type join struct{}

func (s *join) Next(player *model.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	rooms := database.GetRooms()
	buf.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", "ID", "Type", "Players", "State"))
	for _, room := range rooms {
		buf.WriteString(fmt.Sprintf("%d\t%s\t%d\t%s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
	}
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	signal, err := player.AskForString()
	if err != nil {
		return 0, player.WriteError(err)
	}
	if isExit(signal) {
		return s.Exit(player), nil
	}
	if isLs(signal) {
		return consts.StateJoin, nil
	}
	roomId, err := strconv.ParseInt(signal, 10, 64)
	if err != nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	room := database.GetRoom(roomId)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	err = database.JoinRoom(roomId, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	database.RoomBroadcast(roomId, fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players))
	return consts.StateWaiting, nil
}

func (*join) Exit(player *model.Player) consts.StateID {
	return consts.StateHome
}
