package render

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func HomeOptions(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString("1.Join\n")
	buf.WriteString("2.New\n")
	return player.WriteObject(model.Options{
		Options: []model.Option{
			{ID: 1, Name: "Join"},
			{ID: 2, Name: "New"},
		},
	})
}

func GameTypeOptions(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString("Please select game type\n")
	options := make([]model.Option, 0)
	for _, id := range consts.GameTypesIds {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, consts.GameTypes[id]))
		options = append(options, model.Option{ID: id, Name: consts.GameTypes[id]})
	}
	return player.WriteObject(model.Options{
		Options: options,
	})
}

func RoomList(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-10s%-10s%-10s%-10s\n", "ID", "Type", "Players", "State"))
	for _, room := range database.GetRooms() {
		buf.WriteString(fmt.Sprintf("%-10d%-10s%-10d%-10s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
	}
	modelRooms := make([]model.Room, 0)
	for _, room := range database.GetRooms() {
		modelRooms = append(modelRooms, room.Model())
	}
	return player.WriteObject(model.RoomList{
		Rooms: modelRooms,
	})
}

func RoomInfo(player *database.Player, room *database.Room) error {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-20s%-10s%-10s\n", "Name", "Score", "Title"))
	for playerId := range database.GetRoomPlayers(room.ID) {
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
	database.BroadcastObject(room.ID, model.RoomEvent{
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func Exit(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func Offline(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func OwnerChange(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Room:   room.Model(),
		Player: player.Model(),
	})
}
