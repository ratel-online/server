package database

import (
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
	"sync"
	"sync/atomic"
)

var roomIds int64 = 0
var players = map[int64]*model.Player{}
var rooms = map[int64]*model.Room{}
var roomLocks = map[int64]*sync.Mutex{}
var roomPlayers = map[int64]map[int64]bool{}

func PlayerConnected(conn *network.Conn, info *modelx.AuthInfo) *model.Player {
	player := &model.Player{
		ID:    info.ID,
		Name:  info.Name,
		Score: info.Score,
	}
	player.Conn(conn)
	players[conn.ID()] = player
	return player
}

func PlayerDisconnected(conn *network.Conn) {
	delete(players, conn.ID())
}

func CreateRoom(creator int64) *model.Room {
	room := &model.Room{
		ID:      atomic.AddInt64(&roomIds, 1),
		Type:    consts.GameTypeClassic,
		State:   consts.RoomStateWaiting,
		Creator: creator,
	}
	rooms[room.ID] = room
	roomLocks[room.ID] = &sync.Mutex{}
	return room
}

func LockRoom(roomId int64) {
	roomLocks[roomId].Lock()
}

func UnlockRoom(roomId int64) {
	roomLocks[roomId].Unlock()
}

func GetRooms() []*model.Room {
	list := make([]*model.Room, 0)
	for _, room := range rooms {
		list = append(list, room)
	}
	return list
}

func GetRoom(roomId int64) *model.Room {
	return rooms[roomId]
}

func JoinRoom(roomId, playerId int64) error {
	player := players[playerId]
	if player == nil {
		return consts.ErrorsExist
	}
	room := rooms[roomId]
	if room == nil {
		return consts.ErrorsRoomInvalid
	}
	LockRoom(roomId)
	defer UnlockRoom(roomId)
	if room.State == consts.RoomStateRunning {
		return consts.ErrorsJoinFailForRoomRunning
	}
	if GetRoomPlayers(roomId) >= room.Players {
		return consts.ErrorsRoomPlayersIsFull
	}
	roomPlayers[roomId][playerId] = true
	player.RoomID = roomId
	return nil
}

func LeaveRoom(roomId, playerId int64) error {
	player := players[playerId]
	if player == nil {
		return nil
	}
	room := rooms[roomId]
	if room == nil {
		return nil
	}
	LockRoom(roomId)
	defer UnlockRoom(roomId)
	delete(roomPlayers[roomId], playerId)
	player.RoomID = 0
	return nil
}

func GetRoomPlayers(roomId int64) int {
	return len(roomPlayers[roomId])
}

func RoomBroadcast(roomId int64, msg string, exclude ...int64) error {
	room := rooms[roomId]
	if room == nil {
		return consts.ErrorsRoomInvalid
	}
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	for playerId := range roomPlayers[roomId] {
		if player, ok := players[playerId]; ok && !excludeSet[playerId] {
			_ = player.WriteString(msg)
		}
	}
	return nil
}
