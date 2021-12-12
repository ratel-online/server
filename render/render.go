package render

import (
	"bytes"
	"fmt"
	constx "github.com/ratel-online/core/consts"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func Welcome(player *database.Player) error {
	return player.WriteObject(model.Data{
		Code: constx.CodeWelcome,
		Msg:  fmt.Sprintf("Hi %s, Welcome to ratel online! \n", player.Name),
	})
}

func HomeOptions(player *database.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString("1.Join\n")
	buf.WriteString("2.New\n")
	return player.WriteObject(model.Options{
		Data: model.Data{
			Code: constx.CodeHomeOptions,
			Msg:  buf.String(),
		},
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
		Data: model.Data{
			Code: constx.CodeGameTypeOptions,
			Msg:  buf.String(),
		},
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
		Data: model.Data{
			Code: constx.CodeRoomList,
			Msg:  buf.String(),
		},
		Rooms: modelRooms,
	})
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
	database.BroadcastObject(room.ID, model.RoomEvent{
		Data: model.Data{
			Code: constx.CodeRoomEventJoin,
			Msg:  fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players),
		},
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func Exit(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Data: model.Data{
			Code: constx.CodeRoomEventExit,
			Msg:  fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, room.Players),
		},
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func Offline(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Data: model.Data{
			Code: constx.CodeRoomEventOffline,
			Msg:  fmt.Sprintf("%s lost connection", player.Name),
		},
		Room:   room.Model(),
		Player: player.Model(),
	})
}

func OwnerChange(player *database.Player, room *database.Room) {
	database.BroadcastObject(room.ID, model.RoomEvent{
		Data: model.Data{
			Code: constx.CodeRoomEventOwnerChange,
			Msg:  fmt.Sprintf("%s become new owner\n", player.Name),
		},
		Room:   room.Model(),
		Player: player.Model(),
	})
}
