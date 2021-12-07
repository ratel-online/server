package database

import (
	"fmt"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
	"sync"
	"sync/atomic"
)

var roomIds int64 = 0
var players = map[int64]*model.Player{}
var connPlayers = map[int64]*model.Player{}
var rooms = map[int64]*model.Room{}
var roomLocks = map[int64]*sync.Mutex{}
var roomPlayers = map[int64]map[int64]bool{}

//func init() {
//	async.Async(func() {
//		for {
//			time.Sleep(10 * time.Second)
//			log.Infof("current conn %d\n", len(connPlayers))
//		}
//	})
//}

func PlayerConnected(conn *network.Conn, info *modelx.AuthInfo) *model.Player {
	player, ok := players[info.ID]
	if !ok {
		player = &model.Player{
			ID:    info.ID,
			Name:  info.Name,
			Score: info.Score,
		}
	}
	player.Conn(conn)
	players[info.ID] = player
	connPlayers[conn.ID()] = player
	return player
}

func PlayerDisconnected(conn *network.Conn) {
	player := connPlayers[conn.ID()]
	if player != nil {
		player.State(consts.StateWelcome)
		RoomBroadcast(player.RoomID, fmt.Sprintf("%s lost connection!\n", player.Name))
	}

	delete(connPlayers, conn.ID())
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
	roomPlayers[room.ID] = map[int64]bool{}
	return room
}

func DeleteRoom(roomId int64) error {
	delete(rooms, roomId)
	delete(roomLocks, roomId)
	delete(roomPlayers, roomId)
	return nil
}

func GetLock(roomId int64) *sync.Mutex {
	return roomLocks[roomId]
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

func UpdateRoom(room *model.Room) error {
	return nil
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
	lock := GetLock(roomId)
	lock.Lock()
	defer lock.Unlock()
	if room.State == consts.RoomStateRunning {
		return consts.ErrorsJoinFailForRoomRunning
	}
	if room.Players >= consts.MaxPlayers {
		return consts.ErrorsRoomPlayersIsFull
	}
	room.Players++
	roomPlayers[roomId][playerId] = true
	player.RoomID = roomId
	return nil
}

func LeaveRoom(roomId, playerId int64) {
	player := players[playerId]
	if player == nil {
		return
	}
	room := rooms[roomId]
	if room == nil {
		return
	}
	lock := GetLock(roomId)
	lock.Lock()
	defer lock.Unlock()
	player.RoomID = 0
	playerIds := roomPlayers[roomId]
	delete(roomPlayers[roomId], playerId)
	if len(playerIds) == 0 {
		_ = DeleteRoom(roomId)
	}
	if len(playerIds) > 0 {
		for k := range playerIds {
			room.Creator = k
			break
		}
	}
	return
}

func GetRoomPlayers(roomId int64) map[int64]bool {
	return roomPlayers[roomId]
}

func RoomBroadcast(roomId int64, msg string, exclude ...int64) {
	room := rooms[roomId]
	if room == nil {
		return
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
}

func GetPlayer(playerId int64) *model.Player {
	return players[playerId]
}
