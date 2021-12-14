package service

import (
	"github.com/ratel-online/core/consts"
	"github.com/ratel-online/core/errors"
	"github.com/ratel-online/core/model"
	constx "github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type servlet func(player *database.Player, data []byte) model.Resp

var servlets = map[int]servlet{
	consts.ServiceGetRoom:            getRoom,
	consts.ServiceGetRooms:           getRooms,
	consts.ServiceGetRoomPlayers:     getRoomPlayers,
	consts.ServiceGetGame:            getGame,
	consts.ServiceGetGameTypeOptions: getGameTypeOptions,
}

func getRooms(player *database.Player, data []byte) model.Resp {
	modelRooms := make([]model.Room, 0)
	for _, room := range database.GetRooms() {
		modelRooms = append(modelRooms, room.Model())
	}
	return model.SucResp(consts.Service, modelRooms)
}

func getRoom(player *database.Player, data []byte) model.Resp {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	return model.SucResp(consts.Service, room.Model())
}

func getRoomPlayers(player *database.Player, data []byte) model.Resp {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	playerIds := database.GetRoomPlayers(room.ID)
	modelPlayers := make([]model.Player, 0)
	for playerId := range playerIds {
		modelPlayers = append(modelPlayers, database.GetPlayer(playerId).Model())
	}
	return model.SucResp(consts.Service, modelPlayers)
}

func getGameTypeOptions(player *database.Player, data []byte) model.Resp {
	options := make([]model.Option, 0)
	for _, id := range constx.GameTypesIds {
		options = append(options, model.Option{ID: id, Name: constx.GameTypes[id]})
	}
	return model.SucResp(consts.Service, options)
}

func getGame(player *database.Player, data []byte) model.Resp {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	game := room.Game
	if game == nil {
		return model.ErrResp(consts.Service, errors.RoomNotInPlay)
	}
	return model.SucResp(consts.Service, game.Model())
}
