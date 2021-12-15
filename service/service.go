package service

import (
	"github.com/ratel-online/core/consts"
	"github.com/ratel-online/core/errors"
	"github.com/ratel-online/core/model"
	constx "github.com/ratel-online/server/consts"
)

type servlet func(player *Player, req model.Req) model.Resp

type _servlets struct {
	mapping map[int]servlet
}

var servlets = _servlets{}

func init() {
	servlets.mapping = map[int]servlet{
		consts.ServiceGetRoom:        servlets.getRoom,
		consts.ServiceGetRooms:       servlets.getRooms,
		consts.ServiceGetRoomPlayers: servlets.getRoomPlayers,
		consts.ServiceGetGame:        servlets.getGame,
		consts.ServiceGetGameTypes:   servlets.getGameTypes,
		consts.ServiceJoinRoom:       servlets.joinRoom,
		consts.ServiceLeaveRoom:      servlets.leaveRoom,
		consts.ServiceCreateRoom:     servlets.createRoom,
	}
}

func (s _servlets) handle(player *Player, req model.Req) model.Resp {
	if slt, ok := s.mapping[req.Type]; ok {
		return slt(player, req)
	}
	return model.ErrResp(consts.Service, errors.RoomInvalid)
}

func (_servlets) getRooms(player *Player, req model.Req) model.Resp {
	modelRooms := make([]model.Room, 0)
	for _, room := range GetRooms() {
		modelRooms = append(modelRooms, room.Model())
	}
	return model.SucResp(consts.Service, modelRooms)
}

func (_servlets) getRoom(player *Player, req model.Req) model.Resp {
	room := GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	return model.SucResp(consts.Service, room.Model())
}

func (_servlets) createRoom(player *Player, req model.Req) model.Resp {
	gameType := req.Int()
	if _, ok := constx.GameTypes[gameType]; !ok {
		return model.ErrResp(consts.Service, errors.GameTypeInvalid)
	}
	room := createRoom(player.ID)
	room.Type = gameType
	broadcast(player.RoomID, consts.BroadcastCodeRoomEventJoin, model.RoomEvent{
		Player: player.Model(),
		Room:   room.Model(),
	}, player.ID)
	return model.SucResp(consts.Service, room.Model())
}

func (_servlets) joinRoom(player *Player, req model.Req) model.Resp {
	room := getRoom(req.Int64())
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	err := joinRoom(room.ID, player.ID)
	if err != nil {
		return model.ErrResp(consts.Service, err)
	}
	broadcast(room.ID, consts.BroadcastCodeRoomEventJoin, model.RoomEvent{
		Player: player.Model(),
		Room:   room.Model(),
	}, player.ID)
	return model.SucResp(consts.Service, room.Model())
}

func (_servlets) leaveRoom(player *Player, req model.Req) model.Resp {
	room := getRoom(player.RoomID)
	if room != nil && leaveRoom(player.ID, player.RoomID) {
		broadcast(player.RoomID, consts.BroadcastCodeRoomEventJoin, model.RoomEvent{
			Player: player.Model(),
			Room:   room.Model(),
		}, player.ID)
	}
	return model.SucResp(consts.Service, nil)
}

func (_servlets) getRoomPlayers(player *Player, req model.Req) model.Resp {
	room := GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	playerIds := GetRoomPlayers(room.ID)
	modelPlayers := make([]model.Player, 0)
	for playerId := range playerIds {
		modelPlayers = append(modelPlayers, GetPlayer(playerId).Model())
	}
	return model.SucResp(consts.Service, modelPlayers)
}

func (_servlets) getGameTypes(player *Player, req model.Req) model.Resp {
	options := make([]model.Option, 0)
	for _, id := range constx.GameTypesIds {
		options = append(options, model.Option{ID: id, Name: constx.GameTypes[id]})
	}
	return model.SucResp(consts.Service, options)
}

func (_servlets) getGame(player *Player, req model.Req) model.Resp {
	room := GetRoom(player.RoomID)
	if room == nil {
		return model.ErrResp(consts.Service, errors.RoomInvalid)
	}
	game := room.Game
	if game == nil {
		return model.ErrResp(consts.Service, errors.RoomNotInPlay)
	}
	return model.SucResp(consts.Service, game.Model())
}
