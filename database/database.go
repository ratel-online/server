package database

import (
	"fmt"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/server/consts"
	"sort"
	"sync/atomic"
)

var roomIds int64 = 0
var players = map[int64]*Player{}
var connPlayers = map[int64]*Player{}
var rooms = map[int64]*Room{}
var roomPlayers = map[int64]map[int64]bool{}

//func init() {
//	async.Async(func() {
//		for {
//			time.Sleep(10 * time.Second)
//			log.Infof("current conn %d\n", len(connPlayers))
//		}
//	})
//}

func PlayerConnected(conn *network.Conn, info *modelx.AuthInfo) *Player {
	player, ok := players[info.ID]
	if !ok {
		player = &Player{
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
		player.state = consts.StateWelcome
		close(player.data)
		Broadcast(player.RoomID, fmt.Sprintf("%s lost connection!\n", player.Name))
		offline(player)
	}
	delete(connPlayers, conn.ID())
}

func CreateRoom(creator int64) *Room {
	room := &Room{
		ID:      atomic.AddInt64(&roomIds, 1),
		Type:    consts.GameTypeClassic,
		State:   consts.RoomStateWaiting,
		Creator: creator,
	}
	rooms[room.ID] = room
	roomPlayers[room.ID] = map[int64]bool{}
	return room
}

func deleteRoom(room *Room) {
	if room != nil {
		delete(rooms, room.ID)
		delete(roomPlayers, room.ID)
		deleteGame(room.Game)
	}
}

func deleteGame(game *Game) {
	if game != nil {
		for _, state := range game.States {
			close(state)
		}
	}
}

func GetRooms() []*Room {
	list := make([]*Room, 0)
	for _, room := range rooms {
		list = append(list, room)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

func GetRoom(roomId int64) *Room {
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
	room.Lock()
	defer room.Unlock()
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
	room := rooms[roomId]
	if room != nil {
		room.Lock()
		defer room.Unlock()
		leaveRoom(room, players[playerId])
	}
}

func leaveRoom(room *Room, player *Player) {
	if room == nil || player == nil {
		return
	}
	player.RoomID = 0
	room.Players--
	delete(roomPlayers[room.ID], player.ID)
	if len(roomPlayers[room.ID]) == 0 {
		deleteRoom(room)
	}
	if len(roomPlayers[room.ID]) > 0 && room.Creator == player.ID {
		for k := range roomPlayers[room.ID] {
			room.Creator = k
			break
		}
	}
	return
}

func offline(player *Player) {
	room := rooms[player.RoomID]
	if room != nil {
		room.Lock()
		defer room.Unlock()
		if room.State == consts.RoomStateWaiting {
			leaveRoom(room, player)
			return
		}
		if room.State == consts.RoomStateRunning {
			living := false
			for id := range roomPlayers[room.ID] {
				if players[id].IsOnline() {
					living = true
					break
				}
			}
			if !living {
				deleteRoom(room)
			}
		}
	}
}

func RoomPlayers(roomId int64) map[int64]bool {
	return roomPlayers[roomId]
}

func Broadcast(roomId int64, msg string, exclude ...int64) {
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

func GetPlayer(playerId int64) *Player {
	return players[playerId]
}
