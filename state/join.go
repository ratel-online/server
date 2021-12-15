package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/service"
	"strconv"
)

type join struct{}

func (s *join) Next(player *service.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	rooms := service.GetRooms()
	buf.WriteString(fmt.Sprintf("%-10s%-10s%-10s%-10s\n", "ID", "Type", "Players", "State"))
	for _, room := range rooms {
		buf.WriteString(fmt.Sprintf("%-10d%-10s%-10d%-10s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
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
	room := service.GetRoom(roomId)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	err = service.joinRoom(roomId, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	service.broadcast(roomId, fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players))
	return consts.StateWaiting, nil
}

func (*join) Exit(player *service.Player) consts.StateID {
	return consts.StateHome
}
