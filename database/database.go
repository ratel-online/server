package database

import (
	"fmt"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/core/util/strings"
	"github.com/ratel-online/server/consts"
)

var roomIds int64 = 0
var players = newPlayerStore() // 存储连接过服务器的全部用户
var connPlayers = newPlayerStore()
var rooms = newRoomStore()
var roomPlayers = newMemberStore()
var roomSpectators = newMemberStore()
var roomKickedPlayers = newKickedStore()
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
		if r.Type == consts.GameTypeMahjong {
			if n < 3 || n > 4 {
				n = 4
			}
			r.MaxPlayers = n
			return
		}
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
	consts.RoomPropsUndercoverNum: func(r *Room, v string) {
		n, _ := strconv.Atoi(v)
		if n < 1 {
			n = 1
		}
		if n >= r.Players {
			n = r.Players - 1
		}
		r.UndercoverNum = n
	},
	consts.RoomPropsBlankWordMode: func(r *Room, v string) {
		r.BlankWordMode = v == "on"
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
			for _, room := range rooms.Values() {
				roomCancel(room)
			}
			Maintenance() // 清理过期的禁言/封禁/限流/名字占用
		}
	})
}

// Connected 建立玩家对象。名字会被净化并追加 "#短ID"。
// 同一 IP 下同名且对方仍在线时执行"顶号"：踢掉旧连接，新连接沿用旧玩家的 ID 与全部状态。
// 返回的 ConnState 是本次连接的那一代，供读取/关闭时精确对应，避免顶号时误伤新连接。
func Connected(conn *network.Conn, info *modelx.AuthInfo) (*Player, *ConnState, bool, error) {
	player, cs, tookOver, err := claimIdentity(conn, info.Name)
	if err != nil {
		return nil, nil, false, err
	}
	players.Set(player.ID, player)     // 写入用户池（顶号时 ID 不变，覆盖的是同一个玩家）
	connPlayers.Set(conn.ID(), player) // 写入连接用户池
	return player, cs, tookOver, nil
}

// ResumeState 顶号/重连后新连接应该从哪个状态继续：
// 等待房间是无状态的输入循环，可以原地恢复；牌局进行中且座位还在，
// 直接回到牌桌；已经不在房间里的，回大厅。
func ResumeState(p *Player) consts.StateID {
	room := getRoom(p.RoomID)
	if room == nil || p.Role == RoleSpectator || !IsValidPlayer(room.ID, p.ID) {
		return consts.StateWelcome
	}
	if room.State == consts.RoomStateWaiting {
		return consts.StateWaiting
	}
	return consts.GameState(room.Type)
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
	case consts.GameTypeMahjong:
		// 大众麻将支持 3 人或 4 人，默认 4 人
		room.MaxPlayers = 4
	case consts.GameTypeLiar:
		room.MaxPlayers = 4
		room.EnableJokerAsTarget = true
	case consts.GameTypeUndercover:
		room.MaxPlayers = 6
		room.UndercoverNum = 1
		room.BlankWordMode = false
	case consts.GameTypeGouji:
		// 够级固定 6 人
		room.MaxPlayers = 6
	}
	roomPlayers.Set(room.ID, newMemberSet())
	roomSpectators.Set(room.ID, newMemberSet())
	room.Ready = map[int64]bool{}
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
	list := rooms.Values()
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
		return v
	}
	return nil
}

func getPlayer(playerId int64) *Player {
	if v, ok := players.Get(playerId); ok {
		return v
	}
	return nil
}

func SetRoomProps(room *Room, k, v string) {
	// 根据房间类型限制可设置的属性
	allowedProps := getAllowedPropsByGameType(room.Info().Type)

	// 检查属性是否允许设置
	if !allowedProps[k] {
		return // 不允许的属性直接返回，不执行设置
	}

	if setter, ok := roomPropsSetter[k]; ok {
		room.Lock()
		setter(room, v)
		room.Unlock()
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
			consts.RoomPropsPassword:      true,
		}
	case consts.GameTypeUndercover:
		// 对于谁是卧底，允许设置玩家数量、卧底数量、空白词模式和显示IP
		return map[string]bool{
			consts.RoomPropsPlayerNum:     true,
			consts.RoomPropsUndercoverNum: true,
			consts.RoomPropsBlankWordMode: true,
			consts.RoomPropsShowIP:        true,
			consts.RoomPropsPassword:      true,
		}
	case consts.GameTypeUno, consts.GameTypeMahjong:
		// 对于Uno和麻将，允许设置玩家数量和显示IP
		return map[string]bool{
			consts.RoomPropsPlayerNum: true,
			consts.RoomPropsShowIP:    true,
			consts.RoomPropsPassword:  true,
		}
	case consts.GameTypeTexas:
		// 对于德州扑克，允许设置玩家数量和显示IP
		return map[string]bool{
			consts.RoomPropsPlayerNum: true,
			consts.RoomPropsShowIP:    true,
			consts.RoomPropsPassword:  true,
		}
	case consts.GameTypeGouji:
		// 够级人数固定为 6 人，只允许设置密码与显示 IP
		return map[string]bool{
			consts.RoomPropsPassword: true,
			consts.RoomPropsShowIP:   true,
		}
	default:
		// 其他游戏类型允许所有常规属性
		return map[string]bool{
			consts.RoomPropsLaiZi:      true,
			consts.RoomPropsDotShuffle: true,
			consts.RoomPropsSkill:      true,
			consts.RoomPropsPassword:   true,
			consts.RoomPropsPlayerNum:  true,
			consts.RoomPropsChat:       true,
			consts.RoomPropsShowIP:     true,
		}
	}
}

func getRoomPlayers(roomId int64) *memberSet {
	if v, ok := roomPlayers.Get(roomId); ok {
		return v
	}
	return nil
}

func getRoomSpectators(roomId int64) *memberSet {
	if v, ok := roomSpectators.Get(roomId); ok {
		return v
	}
	return nil
}

func IsValidPlayer(roomId, playerId int64) bool {
	room := getRoom(roomId)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		if playersIds := getRoomPlayers(roomId); playersIds != nil && playersIds.Has(playerId) {
			return true
		}
		if spectatorsIds := getRoomSpectators(roomId); spectatorsIds != nil && spectatorsIds.Has(playerId) {
			return true
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
		getRoomSpectators(roomId).Add(playerId)
		player.SetRoom(roomId, RoleSpectator)
	} else {
		getRoomPlayers(roomId).Add(playerId)
		room.Players++
		role := Role(RolePlayer)
		if room.Creator == playerId {
			role = RoleOwner
		}
		player.SetRoom(roomId, role)
	}
	SyncReady(room)
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
	if spectatorsIds.Len() == 0 {
		return nil
	}
	spectators := make([]struct {
		id    int64
		index int
	}, 0)

	for id, index := range spectatorsIds.Copy() {
		spectators = append(spectators, struct {
			id    int64
			index int
		}{id: id, index: index})
	}
	sort.Slice(spectators, func(i, j int) bool {
		return spectators[i].index < spectators[j].index
	})

	playerId := spectators[0].id

	spectatorsIds.Del(playerId)
	getRoomPlayers(room.ID).Add(playerId)
	room.Players++
	player := getPlayer(playerId)
	if player != nil {
		player.SetRole(RolePlayer)
	}
	SyncReady(room)
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
		kickedPlayers[playerId] = true
	}
}

func hasKicked(roomId, playerId int64) bool {
	kickedPlayers, ok := roomKickedPlayers.Get(roomId)
	if !ok {
		return false
	}
	_, exists := kickedPlayers[playerId]
	return exists
}

func leaveRoom(room *Room, player *Player) {
	if room == nil || player == nil {
		return
	}
	room.ActiveTime = time.Now()
	playersIds := getRoomPlayers(room.ID)
	if playersIds.Has(player.ID) {
		room.Players--
		player.SetRoom(0, "")
		playersIds.Del(player.ID)
		if playersIds.Len() > 0 && room.Creator == player.ID {
			for _, k := range playersIds.IDs() {
				room.Creator = k
				if p := getPlayer(k); p != nil {
					p.SetRole(RoleOwner)
				}
				break
			}
		}
	}
	spectatorsIds := getRoomSpectators(room.ID)
	if spectatorsIds.Has(player.ID) {
		player.SetRoom(0, "")
		spectatorsIds.Del(player.ID)
	}
	delete(room.Ready, player.ID)
	SyncReady(room)
	if playersIds.Len() == 0 && spectatorsIds.Len() == 0 {
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
	for _, id := range getRoomPlayers(room.ID).IDs() {
		if p := getPlayer(id); p != nil && p.IsOnline() {
			living = true
			break
		}
	}
	for _, id := range getRoomSpectators(room.ID).IDs() {
		if p := getPlayer(id); p != nil && p.IsOnline() {
			living = true
			break
		}
	}
	if !living {
		log.Infof("room %d is not living, removed.\n", room.ID)
		deleteRoom(room)
	}
}

// RoomPlayers 返回座位上玩家的快照，调用方随便 range，不会和进房/离房冲突。
func RoomPlayers(roomId int64) map[int64]bool {
	set := getRoomPlayers(roomId)
	if set == nil {
		return nil
	}
	out := make(map[int64]bool, set.Len())
	for _, id := range set.IDs() {
		out[id] = true
	}
	return out
}

// RoomSpectators 返回旁观者的快照（值为加入顺序）。
func RoomSpectators(roomId int64) map[int64]int {
	set := getRoomSpectators(roomId)
	if set == nil {
		return nil
	}
	return set.Copy()
}

func broadcast(room *Room, msg string, exclude ...int64) {
	room.ActiveTime = time.Now()
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	// 使用 "[系统] " 前缀而不是 ">> "，避免与输入提示符冲突
	formattedMsg := ">> " + msg
	for _, playerId := range getRoomPlayers(room.ID).IDs() {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(formattedMsg)
		}
	}
	for _, playerId := range getRoomSpectators(room.ID).IDs() {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(formattedMsg)
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

// BroadcastChat 统一聊天出口：所有聊天消息都必须经过这里，
// 以便屏蔽脏词、限流、去重、禁言等防护对等待房间和所有玩法同时生效。
// prefix 是调用方拼好的「名字 [角色] say: 」，content 才是玩家输入。
// 只有 content 过词表，名字里带 "bi"/"si" 这种子串不会被洗掉。
// 返回 false 表示本条消息被拦截（脏话被掩码但已计入违规、刷屏、重复、禁言）。
func BroadcastChat(player *Player, prefix, content string, exclude ...int64) bool {
	if player == nil {
		return false
	}
	if remain := MutedFor(player); remain > 0 {
		_ = player.WriteString(fmt.Sprintf("You are muted, %d second(s) left. \n", int(remain.Seconds())+1))
		return false
	}

	// 归一化后匹配脏词，命中就把这段替换成 '*'，并记一次违规
	content, hits := sanitizeChat(content)
	if hits > 0 {
		Strike(player, "abusive language")
	}
	content = strings.Desensitize(content)
	// 压成单行：内容里的换行会顶出一片空行，也会把别人的消息挤成一行
	content = flattenChatLine(content)
	if content == "" {
		return false
	}

	switch chatCheck(player.ID, content) {
	case chatDuplicate:
		// 最近 5 条内的重复消息直接丢弃，不记违规（正常聊天也会复读）
		return false
	case chatThrottled:
		_ = player.WriteString("Chat throttled, please slow down. \n")
		return false
	}

	log.Infof("chat msg, player %s[%d] %s say: %s\n", player.Name, player.ID, player.IP, content)
	// 每条聊天固定以且仅以一个换行结尾，客户端才能正确换行
	Broadcast(player.RoomID, prefix+content+"\n", exclude...)
	return true
}

// ============================================================================
// 满员 ready 机制：房间满员后 30 秒内必须 ready，否则踢出，避免占座挂机。
// ============================================================================

// ReadyInfo 是一次 ready 状态查询的结果。
type ReadyInfo struct {
	Required bool // 房间满员且正在倒计时
	Ready    bool // 本人是否已 ready
	Expired  bool // 倒计时是否已结束
	AllReady bool // 全员 ready
	ReadyNum int
	Total    int
	Remain   int // 剩余秒数
}

// ReadyStatus 查询某个玩家视角的 ready 状态（内部加房间锁）。
func ReadyStatus(room *Room, playerID int64) ReadyInfo {
	if room == nil {
		return ReadyInfo{}
	}
	room.Lock()
	defer room.Unlock()
	return readyStatusLocked(room, playerID)
}

func readyStatusLocked(room *Room, playerID int64) ReadyInfo {
	// 只统计牌座上的玩家，旁观者 ready 不算数
	seated := getRoomPlayers(room.ID)
	readyNum := 0
	for id := range room.Ready {
		if seated.Has(id) {
			readyNum++
		}
	}
	info := ReadyInfo{Total: room.Players, ReadyNum: readyNum}
	if room.Ready[playerID] && seated.Has(playerID) {
		info.Ready = true
	}
	if room.State != consts.RoomStateWaiting || room.Players < room.MaxPlayers || room.ReadyUntil.IsZero() {
		return info
	}
	info.Required = true
	info.AllReady = readyNum >= room.Players
	if remain := time.Until(room.ReadyUntil); remain > 0 {
		info.Remain = int(remain.Seconds()) + 1
	} else {
		info.Expired = true
	}
	return info
}

// SetReady 记录玩家 ready，返回最新状态（内部加房间锁）。
func SetReady(room *Room, playerID int64) ReadyInfo {
	if room == nil {
		return ReadyInfo{}
	}
	room.Lock()
	defer room.Unlock()
	if !getRoomPlayers(room.ID).Has(playerID) {
		return readyStatusLocked(room, playerID) // 旁观者不能 ready
	}
	if room.Ready == nil {
		room.Ready = map[int64]bool{}
	}
	room.Ready[playerID] = true
	return readyStatusLocked(room, playerID)
}

// SyncReady 在人数/状态变化后维护 ready 窗口。调用方需持有房间锁。
func SyncReady(room *Room) {
	if room == nil {
		return
	}
	if room.State == consts.RoomStateWaiting && room.Players > 0 && room.Players >= room.MaxPlayers {
		if room.ReadyUntil.IsZero() {
			room.ReadyUntil = time.Now().Add(consts.ReadyTimeout)
		}
		return
	}
	room.ReadyUntil = time.Time{}
	for id := range room.Ready {
		delete(room.Ready, id)
	}
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
	for _, playerId := range getRoomPlayers(roomId).IDs() {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(string(msg))
		}
	}
}

func GetPlayer(playerId int64) *Player {
	return getPlayer(playerId)
}
