package state

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ratel-online/server/state/game/texas"
	"github.com/spf13/cast"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/rule"
	"github.com/ratel-online/server/state/game"
)

type waiting struct {
	backfillMutex sync.Mutex // 新增：防止并发 Backfill
}

func (s *waiting) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, consts.ErrorsExist
	}
	s.Backfill(room)

	access, err := s.waitingForStart(player, room)
	if err != nil {
		return 0, err
	}
	if access {
		switch room.Type {
		default:
			return consts.StateGame, nil
		case consts.GameTypeRunFast:
			return consts.StateRunFastGame, nil
		case consts.GameTypeUno:
			return consts.StateUnoGame, nil
		case consts.GameTypeMahjong:
			return consts.StateMahjongGame, nil
		case consts.GameTypeTexas:
			return consts.StateTexasGame, nil
		case consts.GameTypeBullfight:
			return consts.StateBullfightGame, nil
		}
	}
	return s.Exit(player), nil
}

// 修复后的 Exit 方法 - 添加锁保护和空指针检查
func (s *waiting) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return consts.StateHome
	}

	room.Lock()
	wasOwner := room.Creator == player.ID
	oldCreator := room.Creator

	// 离开房间
	database.LeaveRoom(room.ID, player.ID)

	// 获取新的房主ID（如果有变化）
	newCreator := room.Creator
	currentPlayers := room.Players
	room.Unlock()

	// 广播退出消息
	database.Broadcast(room.ID, fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, currentPlayers))

	// 如果原来是房主且房主已变更，广播新房主信息
	if wasOwner && newCreator != oldCreator && newCreator != 0 {
		newOwner := database.GetPlayer(newCreator)
		if newOwner != nil {
			database.Broadcast(room.ID, fmt.Sprintf("%s become new owner\n", newOwner.Name))
		}
	}

	// Backfill 补充玩家
	s.Backfill(room)

	return consts.StateHome
}

// 修复后的 Backfill 方法 - 添加互斥锁防止并发问题
func (s *waiting) Backfill(room *database.Room) {
	if room == nil {
		return
	}

	// 使用互斥锁，确保同一时刻只有一个 Backfill 在执行
	s.backfillMutex.Lock()
	defer s.backfillMutex.Unlock()

	// 双重检查，确保在获取锁后状态依然有效
	room.Lock()
	if room.State == consts.RoomStateRunning {
		room.Unlock()
		return
	}
	currentPlayers := room.Players
	maxPlayers := room.MaxPlayers
	roomID := room.ID
	room.Unlock()

	// 检查是否还有空位
	if currentPlayers >= maxPlayers {
		return
	}

	newPlayer := database.Backfill(roomID)
	if newPlayer != nil {
		// 重新获取最新的玩家数量
		room.Lock()
		updatedPlayers := room.Players
		room.Unlock()

		database.Broadcast(roomID, fmt.Sprintf("%s has joined room! room current has %d players\n", newPlayer.Name, updatedPlayers))
	}
}

// 修复后的 Kicking 方法 - 添加空指针检查
func (s *waiting) Kicking(player *database.Player) {
	if player == nil {
		return
	}

	room := database.GetRoom(player.RoomID)
	if room != nil {
		database.Broadcast(room.ID, fmt.Sprintf("%s has been kicked!\n", player.Name))
		database.Kicking(room.ID, player.ID)

		room.Lock()
		currentPlayers := room.Players
		room.Unlock()

		database.Broadcast(room.ID, fmt.Sprintf("room current has %d players\n", currentPlayers))
	}
}

// 修复后的 waitingForStart 方法 - 添加超时保护和死循环检测
func (s *waiting) waitingForStart(player *database.Player, room *database.Room) (bool, error) {
	access := false
	player.StartTransaction()
	defer player.StopTransaction()

	// 超时保护：最多等待 30 分钟
	overallTimeout := time.After(30 * time.Minute)

	// 循环计数器，用于检测异常循环
	loopCount := 0
	maxConsecutiveTimeouts := 60 // 连续超时60次（60秒）后检查玩家状态
	consecutiveTimeouts := 0

	for {
		loopCount++

		// 检查整体超时
		select {
		case <-overallTimeout:
			return false, consts.ErrorsTimeout
		default:
			// 继续执行
		}

		// 每次循环重新获取 room 对象，防止使用过期数据
		room = database.GetRoom(player.RoomID)
		if room == nil {
			return false, consts.ErrorsRoomInvalid
		}

		signal, err := player.AskForStringWithoutTransaction(time.Second)
		if err != nil && err != consts.ErrorsTimeout {
			return access, err
		}

		// 处理超时情况
		if err == consts.ErrorsTimeout {
			consecutiveTimeouts++

			// 连续超时太多次，检查玩家是否还有效
			if consecutiveTimeouts >= maxConsecutiveTimeouts {
				// 检查玩家是否还在房间中
				if !database.IsValidPlayer(room.ID, player.ID) {
					return false, consts.ErrorsPlayerNotInRoom
				}

				// 重置计数器
				consecutiveTimeouts = 0

			}
		} else {
			// 收到了有效输入，重置超时计数器
			consecutiveTimeouts = 0
		}

		// 验证玩家是否还在房间中
		if !database.IsValidPlayer(room.ID, player.ID) {
			return false, consts.ErrorsPlayerNotInRoom
		}

		// 检查游戏是否已开始
		room.Lock()
		roomState := room.State
		playerRole := player.Role
		room.Unlock()

		if roomState == consts.RoomStateRunning && playerRole == database.RolePlayer {
			log.Infof("Player %d game started, entering game state", player.ID)
			access = true
			break
		}

		signal = strings.TrimSpace(strings.ToLower(signal))
		if signal == "" {
			continue
		}

		segments := strings.Split(signal, " ")
		if len(segments) == 1 {
			if segments[0] == "ls" || segments[0] == "v" {
				viewRoomPlayers(room, player)
				continue
			} else if segments[0] == "start" || signal == "s" {
				room.Lock()
				isCreator := room.Creator == player.ID
				currentPlayers := room.Players
				roomType := room.Type
				room.Unlock()

				if isCreator {
					if currentPlayers <= 1 {
						_ = player.WriteError(consts.ErrorsGamePlayersInsufficient)
						continue
					}
					if roomType == consts.GameTypeRunFast && currentPlayers != 3 {
						_ = player.WriteError(consts.ErrorsGamePlayersInvalid)
						continue
					}
					err = startGame(player, room)
					if err != nil {
						log.Errorf("Player %d failed to start game: %v", player.ID, err)
						return access, err
					}
					log.Infof("Player %d started game in room %d", player.ID, room.ID)
					access = true
					break
				}
			}
		} else if len(segments) == 2 {
			if segments[0] == "kicking" || segments[0] == "kill" || segments[0] == "k" {
				room.Lock()
				isCreator := room.Creator == player.ID
				room.Unlock()

				if isCreator {
					kickedId := cast.ToInt64(segments[1])
					if kickedId == player.ID {
						_ = player.WriteError(consts.ErrorsCannotKickYourself)
						continue
					}

					kickedPlayer := database.GetPlayer(kickedId)
					if kickedPlayer == nil {
						_ = player.WriteError(consts.ErrorsGamePlayersInvalid)
						continue
					}

					if kickedPlayer.RoomID != room.ID {
						_ = player.WriteError(consts.ErrorsPlayerNotInRoom)
						continue
					}

					log.Infof("Player %d kicking player %d from room %d", player.ID, kickedId, room.ID)
					s.Kicking(kickedPlayer)
					continue
				}
			}
		} else if len(segments) == 3 {
			room.Lock()
			isCreator := room.Creator == player.ID
			room.Unlock()

			if isCreator {
				database.SetRoomProps(room, segments[1], segments[2])
				continue
			}
		}

		// 处理聊天消息
		room.Lock()
		enableChat := room.EnableChat
		roomState = room.State
		room.Unlock()

		if enableChat {
			if roomState == consts.RoomStateRunning {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopenedDuringGame.Error()))
			} else {
				database.BroadcastChat(player, fmt.Sprintf("%s [%s] say: %s\n", player.Name, player.Role, signal))
			}
		} else {
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
		}
	}

	return access, nil
}

// startGame 方法保持不变，但添加更多日志
func startGame(player *database.Player, room *database.Room) (err error) {
	room.Lock()
	defer room.Unlock()

	switch room.Type {
	default:
		room.Game, err = game.InitGame(room)
	case consts.GameTypeUno:
		room.Game, err = game.InitUnoGame(room)
	case consts.GameTypeRunFast:
		room.Game, err = game.InitRunFastGame(room, rule.RunFastRules)
	case consts.GameTypeMahjong:
		room.Game, err = game.InitMahjongGame(room)
	case consts.GameTypeBullfight:
		room.Game, err = game.InitNiuniuGame(room)
	case consts.GameTypeTexas:
		room.Game, err = texas.Init(room)
	}

	if err != nil {
		log.Errorf("Failed to init game for room %d: %v", room.ID, err)
		_ = player.WriteError(err)
		return err
	}

	room.State = consts.RoomStateRunning
	log.Infof("Game started successfully in room %d", room.ID)
	return nil
}

// viewRoomPlayers 保持不变
func viewRoomPlayers(room *database.Room, currPlayer *database.Player) {
	room.Lock()
	defer room.Unlock()

	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Room ID: %d\n", room.ID))
	buf.WriteString("Players:\n")
	for playerId := range database.RoomPlayers(room.ID) {
		player := database.GetPlayer(playerId)
		if player != nil {
			buf.WriteString(fmt.Sprintf("%s [%s], score: %d, id: %d\n", player.Name, player.Role, player.Amount, player.ID))
		}
	}

	buf.WriteString("\nSpectators:\n")
	for spectatorId := range database.RoomSpectators(room.ID) {
		spectator := database.GetPlayer(spectatorId)
		if spectator != nil {
			buf.WriteString(fmt.Sprintf("%s [spectator], score: %d, id: %d\n", spectator.Name, spectator.Amount, spectator.ID))
		}
	}

	buf.WriteString("\nSettings:\n")
	switch room.Type {
	case consts.GameTypeUno, consts.GameTypeMahjong:
	case consts.GameTypeTexas:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "pn:", room.MaxPlayers))
	default:
		buf.WriteString(fmt.Sprintf("%-5s%-5v%-5s%-5v\n", "lz:", sprintPropsState(room.EnableLaiZi)+",", "ds:", sprintPropsState(room.EnableDontShuffle)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v%-5s%-5v\n", "sk:", sprintPropsState(room.EnableSkill)+",", "pn:", room.MaxPlayers))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ct:", sprintPropsState(room.EnableChat)))
	}
	pwd := room.Password
	if pwd != "" {
		if room.Creator != currPlayer.ID {
			pwd = "********"
		}
	} else {
		pwd = "off"
	}
	buf.WriteString(fmt.Sprintf("%-5s%-20v\n", "pwd", pwd))
	_ = currPlayer.WriteString(buf.String())
}

func sprintPropsState(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
