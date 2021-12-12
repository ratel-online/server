package database

import (
	"fmt"
	"github.com/awesome-cap/hashmap"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/server/consts"
	"sort"
	"sync/atomic"
)

var roomIds int64 = 0
var players = hashmap.New()
var connPlayers = hashmap.New()
var rooms = hashmap.New()
var roomPlayers = hashmap.New()

//func init() {
//	async.Async(func() {
//		for {
//			time.Sleep(10 * time.Second)
//			log.Infof("current conn %d\n", len(connPlayers))
//		}
//	})
//}

func Connected(conn *network.Conn, info *modelx.AuthInfo) *Player {
	player := &Player{
		ID:    info.ID,
		Name:  info.Name,
		Score: info.Score,
	}
	player.Conn(conn)
	players.Set(info.ID, player)
	connPlayers.Set(conn.ID(), player)
	return player
}

func Disconnected(conn *network.Conn) {
	if v, ok := connPlayers.Get(conn.ID()); ok {
		player := v.(*Player)
		roomId := player.RoomID
		player.state = consts.StateWelcome
		player.RoomID = 0
		close(player.data)
		offline(roomId, player.ID)
		Broadcast(roomId, fmt.Sprintf("%s lost connection!\n", player.Name))
	}
	connPlayers.Del(conn.ID())
}

func CreateRoom(creator int64) *Room {
	room := &Room{
		ID:      atomic.AddInt64(&roomIds, 1),
		Type:    consts.GameTypeClassic,
		State:   consts.RoomStateWaiting,
		Creator: creator,
	}
	rooms.Set(room.ID, room)
	roomPlayers.Set(room.ID, map[int64]bool{})
	return room
}

func deleteRoom(room *Room) {
	if room != nil {
		fmt.Println("delete", room.ID)
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

func JoinRoom(roomId, playerId int64) error {
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
	players := getRoomPlayers(roomId)
	if players != nil {
		players[playerId] = true
	}
	room.Players++
	player.RoomID = roomId
	return nil
}

func LeaveRoom(roomId, playerId int64) {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		leaveRoom(room, getPlayer(playerId))
	}
}

func leaveRoom(room *Room, player *Player) {
	if room == nil || player == nil {
		return
	}
	roomPlayers := getRoomPlayers(room.ID)
	room.Players--
	delete(roomPlayers, player.ID)
	if len(roomPlayers) == 0 {
		deleteRoom(room)
	}
	if len(roomPlayers) > 0 && room.Creator == player.ID {
		for k := range roomPlayers {
			room.Creator = k
			break
		}
	}
	return
}

func offline(roomId, playerId int64) {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		if room.State == consts.RoomStateWaiting {
			leaveRoom(room, getPlayer(playerId))
			return
		}
		if room.State == consts.RoomStateRunning {
			living := false
			roomPlayers := getRoomPlayers(room.ID)
			for id := range roomPlayers {
				if getPlayer(id).online {
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
	return getRoomPlayers(roomId)
}

func Broadcast(roomId int64, msg string, exclude ...int64) {
	room := getRoom(roomId)
	if room == nil {
		return
	}
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	roomPlayers := getRoomPlayers(roomId)
	for playerId := range roomPlayers {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(msg)
		}
	}
}

func BroadcastObject(roomId int64, object interface{}, exclude ...int64) {
	room := getRoom(roomId)
	if room == nil {
		return
	}
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	msg := json.Marshal(object)
	roomPlayers := getRoomPlayers(roomId)
	for playerId := range roomPlayers {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(string(msg))
		}
	}
}

func GetPlayer(playerId int64) *Player {
	return getPlayer(playerId)
}
