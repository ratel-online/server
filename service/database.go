package service

import (
	"github.com/awesome-cap/hashmap"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/server/consts"
	"sort"
	"sync/atomic"
	"time"
)

var roomIds int64 = 0
var players = hashmap.New()
var connPlayers = hashmap.New()
var rooms = hashmap.New()
var roomPlayers = hashmap.New()

func init() {
	async.Async(func() {
		for {
			time.Sleep(1 * time.Minute)
			rooms.Foreach(func(e *hashmap.Entry) {
				roomCancel(e.Value().(*Room))
			})
		}
	})
}

func createRoom(creator int64) *Room {
	room := &Room{
		ID:      atomic.AddInt64(&roomIds, 1),
		Type:    consts.GameTypeClassic,
		State:   consts.RoomStateWaiting,
		Creator: creator,
	}
	rooms.Set(room.ID, room)
	roomPlayers.Set(room.ID, map[int64]bool{})
	_ = joinRoom(room.ID, creator)
	return room
}

func deleteRoom(room *Room) {
	if room != nil {
		rooms.Del(room.ID)
		roomPlayers.Del(room.ID)
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
	rooms.Foreach(func(e *hashmap.Entry) {
		list = append(list, e.Value().(*Room))
	})
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

func GetRoom(roomId int64) *Room {
	return getRoom(roomId)
}

func getRoom(roomId int64) *Room {
	if v, ok := rooms.Get(roomId); ok {
		return v.(*Room)
	}
	return nil
}

func getPlayer(playerId int64) *Player {
	if v, ok := players.Get(playerId); ok {
		return v.(*Player)
	}
	return nil
}

func getRoomPlayers(roomId int64) map[int64]bool {
	if v, ok := roomPlayers.Get(roomId); ok {
		return v.(map[int64]bool)
	}
	return nil
}

func joinRoom(roomId, playerId int64) error {
	player := getPlayer(playerId)
	if player == nil {
		return consts.ErrorsExist
	}
	room := getRoom(roomId)
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
	playersIds := getRoomPlayers(roomId)
	if playersIds != nil {
		playersIds[playerId] = true
		room.Players++
		player.RoomID = roomId
	}
	return nil
}

func leaveRoom(roomId, playerId int64) bool {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		return _leaveRoom(room, getPlayer(playerId))
	}
	return false
}

func _leaveRoom(room *Room, player *Player) bool {
	if room == nil || player == nil {
		return false
	}
	playersIds := getRoomPlayers(room.ID)
	_, ok := playersIds[player.ID]
	if ok {
		room.Players--
		player.RoomID = 0
		delete(playersIds, player.ID)
		if len(playersIds) > 0 && room.Creator == player.ID {
			for k := range playersIds {
				room.Creator = k
				break
			}
		}
	}
	if len(playersIds) == 0 {
		deleteRoom(room)
	}
	return ok
}

func offline(roomId, playerId int64) {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		if room.State == consts.RoomStateWaiting {
			_leaveRoom(room, getPlayer(playerId))
		}
		roomCancel(room)
	}
}

func roomCancel(room *Room) {
	living := false
	playerIds := getRoomPlayers(room.ID)
	for id := range playerIds {
		if getPlayer(id).online {
			living = true
			break
		}
	}
	if !living {
		log.Infof("room %d is not living, removed.\n", room.ID)
		deleteRoom(room)
	}
}

func GetRoomPlayers(roomId int64) map[int64]bool {
	return getRoomPlayers(roomId)
}

func broadcast(roomId int64, code int, object interface{}, exclude ...int64) {
	room := getRoom(roomId)
	if room == nil {
		return
	}
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	msg := json.Marshal(model.SucBroadcast(code, object))
	playerIds := getRoomPlayers(roomId)
	for playerId := range playerIds {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(string(msg))
		}
	}
}

func GetPlayer(playerId int64) *Player {
	return getPlayer(playerId)
}
