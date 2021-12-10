package render

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func Welcome(player *database.Player) error {
	return player.WriteString(fmt.Sprintf("Hi %s, Welcome to ratel online! \n", player.Name))
}

func Home(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString("1.Join\n")
	buf.WriteString("2.New\n")
	return player.WriteString(buf.String())
}

func RoomList(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-10s%-10s%-10s%-10s\n", "ID", "Type", "Players", "State"))
	for _, room := range database.GetRooms() {
		buf.WriteString(fmt.Sprintf("%-10d%-10s%-10d%-10s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
	}
	return player.WriteString(buf.String())
}

func RoomInfo(player *database.Player, room *database.Room) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-20s%-10s%-10s\n", "Name", "Score", "Title"))
	for playerId := range database.RoomPlayers(room.ID) {
		title := "player"
		if playerId == room.Creator {
			title = "owner"
		}
		info := database.GetPlayer(playerId)
		buf.WriteString(fmt.Sprintf("%-20s%-10d%-10s\n", info.Name, info.Score, title))
	}
	return player.WriteString(buf.String())
}

func Error(player *database.Player, err error) error {
	return player.WriteError(err)
}

func Join(player *database.Player, room *database.Room) {
	database.Broadcast(room.ID, fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players))
}

func Exit(player *database.Player, room *database.Room) {
	database.Broadcast(room.ID, fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, room.Players))
}

func Offline(player *database.Player, room *database.Room) {
	database.Broadcast(room.ID, fmt.Sprintf("%s lost connection", player.Name))
}
