package database

import (
	"sort"
	"strconv"
	stringx "strings"
	"sync/atomic"
	"time"

	"github.com/awesome-cap/hashmap"
	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/core/util/strings"
	"github.com/ratel-online/server/consts"
)

var roomIds int64 = 0
var players = hashmap.New() // 存储连接过服务器的全部用户
var connPlayers = hashmap.New()
var rooms = hashmap.New()
var roomPlayers = hashmap.New()
var roomSpectators = hashmap.New()
var roomKickedPlayers = hashmap.New()
var roomPropsSetter = map[string]func(r *Room, v string){
	consts.RoomPropsSkill: func(r *Room, v string) {
		r.EnableSkill = v == "on"
		r.EnableLandlord = !r.EnableSkill
	},
	consts.RoomPropsLaiZi: func(r *Room, v string) {
		r.EnableLaiZi = v == "on"
	},
	consts.RoomPropsDotShuffle: func(r *Room, v string) {
		r.EnableDontShuffle = v == "on"
	},
	consts.RoomPropsPassword: func(r *Room, v string) {
		if v == "off" {
			r.Password = ""
		} else {
			r.Password = v
		}
	},
	consts.RoomPropsChat: func(r *Room, v string) {
		r.EnableChat = v == "on"
	},
	consts.RoomPropsPlayerNum: func(r *Room, v string) {
		n, _ := strconv.Atoi(v)
		if n < 2 || n > 50 {
			n = consts.MaxPlayers
		}
		r.MaxPlayers = n
	},
	consts.RoomPropsShowIP: func(r *Room, v string) {
		r.EnableShowIP = v == "on"
	},
	consts.RoomPropsJokerAsTarget: func(r *Room, v string) {
		r.EnableJokerAsTarget = v == "on"
	},
}

func init() {
	async.Async(func() {
		loopCount := 0
		for {
			loopCount++
			if loopCount%60 == 0 {
				log.Infof("[database.init] Room cleanup loop count: %d (running for %d hours)\n", loopCount, loopCount/60)
			}
			time.Sleep(1 * time.Minute)
			rooms.Foreach(func(e *hashmap.Entry) {
				roomCancel(e.Value().(*Room))
			})
		}
	})
}

func Connected(conn *network.Conn, info *modelx.AuthInfo) *Player {
	player := &Player{
		ID:     conn.ID(),
		IP:     conn.IP(),
		Name:   strings.Desensitize(info.Name),
		Amount: 2000,
	}
	player.Conn(conn)                  // 初始化play对象
	players.Set(conn.ID(), player)     // 写入用户池
	connPlayers.Set(conn.ID(), player) // 写入连接用户池
	return player
}

func CreateRoom(creator int64, t int) *Room {
	room := &Room{
		ID:             atomic.AddInt64(&roomIds, 1),
		Type:           t,
		State:          consts.RoomStateWaiting,
		Creator:        creator,
		ActiveTime:     time.Now(),
		MaxPlayers:     consts.MaxPlayers,
		EnableLandlord: true,
		EnableChat:     true,
		EnableShowIP:   false,
	}
	switch room.Type {
	case consts.GameTypeLaiZi:
		room.EnableLaiZi = true
	case consts.GameTypeSkill:
		room.EnableLaiZi = true
		room.EnableDontShuffle = true
		room.EnableSkill = true
		room.EnableLandlord = false
	case consts.GameTypeRunFast:
		room.MaxPlayers = 3
		room.EnableLaiZi = false
		room.EnableLandlord = false
		room.EnableDontShuffle = true
	case consts.GameTypeTexas:
		room.MaxPlayers = 10
	case consts.GameTypeLiar:
		room.MaxPlayers = 4
		room.EnableJokerAsTarget = true
	}
	roomPlayers.Set(room.ID, map[int64]bool{})
	roomSpectators.Set(room.ID, map[int64]int{})
	rooms.Set(room.ID, room)
	return room
}

func deleteRoom(room *Room) {
	if room != nil {
		rooms.Del(room.ID)
		roomPlayers.Del(room.ID)
		roomSpectators.Del(room.ID)
		if room.Game != nil {
			room.Game.Clean()
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

func SetRoomProps(room *Room, k, v string) {
	// 根据房间类型限制可设置的属性
	allowedProps := getAllowedPropsByGameType(room.Type)

	// 检查属性是否允许设置
	if !allowedProps[k] {
		return // 不允许的属性直接返回，不执行设置
	}

	if setter, ok := roomPropsSetter[k]; ok {
		setter(room, v)
	}
}

// 根据游戏类型返回允许设置的属性列表
func getAllowedPropsByGameType(gameType int) map[string]bool {
	switch gameType {
	case consts.GameTypeLiar:
		// 对于骗子酒馆，只允许设置指示牌规则和显示IP
		return map[string]bool{
			consts.RoomPropsJokerAsTarget: true,
			consts.RoomPropsShowIP:        true,
		}
	case consts.GameTypeUno, consts.GameTypeMahjong:
		// 对于Uno和麻将，只允许设置显示IP
		return map[string]bool{
			consts.RoomPropsShowIP: true,
		}
	case consts.GameTypeTexas:
		// 对于德州扑克，允许设置玩家数量和显示IP
		return map[string]bool{
			consts.RoomPropsPlayerNum: true,
			consts.RoomPropsShowIP:    true,
		}
	default:
		// 其他游戏类型允许所有常规属性
		return map[string]bool{
			consts.RoomPropsLaiZi:         true,
			consts.RoomPropsDotShuffle:    true,
			consts.RoomPropsSkill:         true,
			consts.RoomPropsPassword:      true,
			consts.RoomPropsPlayerNum:     true,
			consts.RoomPropsChat:          true,
			consts.RoomPropsShowIP:        true,
			consts.RoomPropsJokerAsTarget: true,
		}
	}
}

func getRoomPlayers(roomId int64) map[int64]bool {
	if v, ok := roomPlayers.Get(roomId); ok {
		return v.(map[int64]bool)
	}
	return nil
}

func getRoomSpectators(roomId int64) map[int64]int {
	if v, ok := roomSpectators.Get(roomId); ok {
		return v.(map[int64]int)
	}
	return nil
}

func IsValidPlayer(roomId, playerId int64) bool {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		playersIds := getRoomPlayers(roomId)
		if playersIds != nil {
			_, exists := playersIds[playerId]
			if exists {
				return true
			}
		}
		spectatorsIds := getRoomSpectators(roomId)
		if spectatorsIds != nil {
			_, exists := spectatorsIds[playerId]
			if exists {
				return true
			}
		}
		return false
	}
	return false
}

// 加入房间
func JoinRoom(roomId, playerId int64) error {
	// 资源检查
	player := getPlayer(playerId)
	if player == nil {
		return consts.ErrorsExist
	}
	room := getRoom(roomId)
	if room == nil {
		return consts.ErrorsRoomInvalid
	}

	// 加锁防止并发异常
	room.Lock()
	defer room.Unlock()

	if hasKicked(roomId, playerId) {
		return consts.ErrorsJoinFailForKicked
	}

	room.ActiveTime = time.Now()

	//房间人数及状态检查
	if room.Players >= room.MaxPlayers || room.State == consts.RoomStateRunning {
		spectatorsIds := getRoomSpectators(roomId)
		spectatorsIds[playerId] = len(spectatorsIds)
		player.RoomID = roomId
		player.Role = RoleSpectator
	} else {
		playersIds := getRoomPlayers(roomId)
		playersIds[playerId] = true
		room.Players++
		player.RoomID = roomId
		player.Role = RolePlayer
		if room.Creator == playerId {
			player.Role = RoleOwner
		}
	}

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

func Backfill(roomId int64) *Player {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		return backfill(room)
	}
	return nil
}

func backfill(room *Room) *Player {
	if room.Players >= room.MaxPlayers {
		return nil
	}
	spectatorsIds := getRoomSpectators(room.ID)
	if len(spectatorsIds) == 0 {
		return nil
	}
	spectators := make([]struct {
		id    int64
		index int
	}, 0)

	for id, index := range spectatorsIds {
		spectators = append(spectators, struct {
			id    int64
			index int
		}{id: id, index: index})
	}
	sort.Slice(spectators, func(i, j int) bool {
		return spectators[i].index < spectators[j].index
	})

	playerId := spectators[0].id

	delete(spectatorsIds, playerId)
	playersIds := getRoomPlayers(room.ID)
	playersIds[playerId] = true
	room.Players++
	player := getPlayer(playerId)
	if player != nil {
		player.Role = RolePlayer
	}
	return player
}

func Kicking(roomId, playerId int64) {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		leaveRoom(room, getPlayer(playerId))

		kickedPlayers, ok := roomKickedPlayers.Get(roomId)
		if !ok {
			kickedPlayers = map[int64]bool{}
			roomKickedPlayers.Set(roomId, kickedPlayers)
		}
		kickedPlayers.(map[int64]bool)[playerId] = true
	}
}

func hasKicked(roomId, playerId int64) bool {
	kickedPlayers, ok := roomKickedPlayers.Get(roomId)
	if !ok {
		return false
	}
	_, exists := kickedPlayers.(map[int64]bool)[playerId]
	return exists
}

func leaveRoom(room *Room, player *Player) {
	if room == nil || player == nil {
		return
	}
	room.ActiveTime = time.Now()
	playersIds := getRoomPlayers(room.ID)
	if _, ok := playersIds[player.ID]; ok {
		room.Players--
		player.RoomID = 0
		player.Role = ""
		delete(playersIds, player.ID)
		if len(playersIds) > 0 && room.Creator == player.ID {
			for k := range playersIds {
				room.Creator = k
				if p := getPlayer(k); p != nil {
					p.Role = RoleOwner
				}
				break
			}
		}
	}
	spectatorsIds := getRoomSpectators(room.ID)
	if _, ok := spectatorsIds[player.ID]; ok {
		player.RoomID = 0
		player.Role = ""
		delete(spectatorsIds, player.ID)
	}
	if len(playersIds) == 0 && len(spectatorsIds) == 0 {
		deleteRoom(room)
	}
}

func roomCancel(room *Room) {
	if room.ActiveTime.Add(24 * time.Hour).Before(time.Now()) {
		log.Infof("room %d is timeout 24 hours, removed.\n", room.ID)
		deleteRoom(room)
		return
	}
	living := false
	playerIds := getRoomPlayers(room.ID)
	for id := range playerIds {
		if getPlayer(id).online {
			living = true
			break
		}
	}
	spectatorIds := getRoomSpectators(room.ID)
	for id := range spectatorIds {
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

func RoomPlayers(roomId int64) map[int64]bool {
	return getRoomPlayers(roomId)
}

func RoomSpectators(roomId int64) map[int64]int {
	return getRoomSpectators(roomId)
}

func broadcast(room *Room, msg string, exclude ...int64) {
	room.ActiveTime = time.Now()
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	for playerId := range getRoomPlayers(room.ID) {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(">> " + msg)
		}
	}
	for playerId := range getRoomSpectators(room.ID) {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(">> " + msg)
		}
	}
}

func Broadcast(roomId int64, msg string, exclude ...int64) {
	room := getRoom(roomId)
	if room == nil {
		return
	}
	broadcast(room, msg, exclude...)
}

func BroadcastChat(player *Player, msg string, exclude ...int64) {
	log.Infof("chat msg, player %s[%d] %s say: %s\n", player.Name, player.ID, player.IP, stringx.TrimSpace(msg))
	Broadcast(player.RoomID, strings.Desensitize(msg), exclude...)
}

func BroadcastObject(roomId int64, object interface{}, exclude ...int64) {
	room := getRoom(roomId)
	if room == nil {
		return
	}
	room.Lock()
	defer room.Unlock()
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	msg := json.Marshal(object)
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
