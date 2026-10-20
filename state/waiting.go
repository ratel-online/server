package state

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/ratel-online/server/state/game/texas"
	"github.com/spf13/cast"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/rule"
	"github.com/ratel-online/server/state/game"
)

type waiting struct{}

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
		switch room.Info().Type {
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
		case consts.GameTypeLiar:
			return consts.StateLiarGame, nil
		case consts.GameTypeUndercover:
			return consts.StateUndercoverGame, nil
		case consts.GameTypeGouji:
			return consts.StateGoujiGame, nil
		}
	}
	return s.Exit(player), nil
}

func (s *waiting) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		info := room.Info()
		isOwner := info.Creator == player.ID
		database.LeaveRoom(room.ID, player.ID)
		database.Broadcast(room.ID, fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, room.Info().Players))
		if isOwner {
			newOwner := database.GetPlayer(room.Info().Creator)
			if newOwner != nil {
				database.Broadcast(room.ID, fmt.Sprintf("%s become new owner\n", newOwner.Name))
			}
		}
		s.Backfill(room)
	}
	return consts.StateHome
}

func (*waiting) Backfill(room *database.Room) {
	if room.Info().State == consts.RoomStateRunning {
		return
	}
	newPlayer := database.Backfill(room.ID)
	if newPlayer != nil {
		database.Broadcast(room.ID, fmt.Sprintf("%s has joined room! room current has %d players\n", newPlayer.Name, room.Info().Players))
	}
}

func (*waiting) Kicking(player *database.Player) {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		database.Broadcast(room.ID, fmt.Sprintf("%s has been kicked!\n", player.Name))
		database.Kicking(room.ID, player.ID)
		database.Broadcast(room.ID, fmt.Sprintf("room current has %d players\n", room.Info().Players))
	}
}

func (s *waiting) waitingForStart(player *database.Player, room *database.Room) (bool, error) {
	access := false
	//对局类别
	player.StartTransaction()
	defer player.StopTransaction()
	loopCount := 0
	for {
		loopCount++
		roomInfo := room.Info()
		if loopCount%100 == 0 {
			log.Infof("[waitingForStart] Player %d (Room %d) loop count: %d, room.State: %d, access: %v\n", player.ID, player.RoomID, loopCount, roomInfo.State, access)
		}

		// 挂机检测：等待房间里太久没有输入，自动离房，别占着座位
		if roomInfo.State == consts.RoomStateWaiting && player.IdleFor() > consts.WaitingIdleTimeout {
			database.Broadcast(room.ID, fmt.Sprintf("%s was idle for %d minutes and left the room.\n",
				player.Name, int(consts.WaitingIdleTimeout.Minutes())))
			database.LeaveRoom(room.ID, player.ID)
			_ = player.WriteString("You were idle for too long and left the room.\n")
			return false, consts.ErrorsPlayerNotInRoom
		}

		// 满员 ready 倒计时：全员 ready 直接开局，超时未 ready 的直接踢
		if info := database.ReadyStatus(room, player.ID); info.Required {
			if info.AllReady {
				if err := startGame(player, room); err != nil {
					_ = player.WriteError(err)
					continue
				}
				access = true
				break
			}
			if info.Expired && !info.Ready {
				database.Broadcast(room.ID, fmt.Sprintf("%s was removed for not readying in %d seconds.\n",
					player.Name, int(consts.ReadyTimeout.Seconds())))
				s.Kicking(player)
				return false, consts.ErrorsExist
			}
		}

		signal, err := player.AskForStringWithoutTransaction(time.Second)
		if err != nil && err != consts.ErrorsTimeout {
			return access, err
		}

		if !database.IsValidPlayer(room.ID, player.ID) {
			return false, consts.ErrorsPlayerNotInRoom
		}

		if room.Info().State == consts.RoomStateRunning && player.Role == database.RolePlayer {
			access = true
			break
		}
		rawSignal := strings.TrimSpace(signal)
		signal = strings.ToLower(rawSignal)
		if signal == "" {
			continue
		}

		segments := strings.Fields(signal)

		if segments[0] == "ls" || segments[0] == "v" {
			viewRoomPlayers(room, player)
			continue
		}
		if segments[0] == "ready" || segments[0] == "r" {
			if info := database.ReadyStatus(room, player.ID); !info.Required {
				_ = player.WriteString("Room is not full yet, no need to ready.\n")
				continue
			}
			info := database.SetReady(room, player.ID)
			database.Broadcast(room.ID, fmt.Sprintf("%s is ready (%d/%d)\n", player.Name, info.ReadyNum, info.Total))
			continue
		}

		// 房主指令：开局 / 踢人 / 禁言 / 解除禁言 / 房间属性
		handled, ok, cmdErr := s.ownerCommand(player, room, segments)
		if handled {
			if cmdErr != nil {
				return access, cmdErr
			}
			if ok {
				access = true
				break
			}
			continue
		}

		if room.EnableChat {
			if room.State == consts.RoomStateRunning {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopenedDuringGame.Error()))
			} else {
				database.BroadcastChat(player, fmt.Sprintf("%s [%s] say: ", player.Name, player.Role), rawSignal)
			}
		} else {
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
		}
	}
	return access, nil
}

// ownerCommand 处理房主指令。handled=false 表示不是房主指令，调用方按聊天处理；
// ok=true 表示已经可以进入牌局。
func (s *waiting) ownerCommand(player *database.Player, room *database.Room, segments []string) (handled, ok bool, err error) {
	info := room.Info()
	if info.Creator != player.ID || len(segments) == 0 {
		return false, false, nil
	}
	switch segments[0] {
	case "start", "s":
		if info.Players <= 1 {
			_ = player.WriteError(consts.ErrorsGamePlayersInsufficient)
			return true, false, nil
		}
		if info.Type == consts.GameTypeRunFast && info.Players != 3 {
			_ = player.WriteError(consts.ErrorsGamePlayersInvalid)
			return true, false, nil
		}
		if info.Type == consts.GameTypeMahjong && (info.Players < 3 || info.Players > 4) {
			_ = player.WriteString(fmt.Sprintf("大众麻将需要 3 或 4 名玩家，当前 %d 人。\n", info.Players))
			return true, false, nil
		}
		if info.Type == consts.GameTypeUndercover && info.Players < 3 {
			_ = player.WriteString("谁是卧底游戏至少需要3名玩家！\n")
			return true, false, nil
		}
		if info.Type == consts.GameTypeGouji && info.Players != 6 {
			_ = player.WriteString(fmt.Sprintf("够级需要 6 名玩家，当前 %d 人。\n", info.Players))
			return true, false, nil
		}
		if err = startGame(player, room); err != nil {
			return true, false, err
		}
		return true, true, nil

	case "k", "kill", "kicking":
		if len(segments) < 2 {
			_ = player.WriteError(consts.ErrorsInputInvalid)
			return true, false, nil
		}
		kickedID := cast.ToInt64(segments[1])
		if kickedID == player.ID {
			_ = player.WriteError(consts.ErrorsCannotKickYourself)
			return true, false, nil
		}
		kicked := database.GetPlayer(kickedID)
		if kicked == nil || kicked.RoomID != room.ID {
			_ = player.WriteError(consts.ErrorsPlayerNotInRoom)
			return true, false, nil
		}
		s.Kicking(kicked)
		return true, false, nil

	case "mute":
		if len(segments) < 2 {
			_ = player.WriteString("Usage: mute <id> [minutes]\n")
			return true, false, nil
		}
		target := database.GetPlayer(cast.ToInt64(segments[1]))
		if target == nil || target.RoomID != room.ID {
			_ = player.WriteError(consts.ErrorsPlayerNotInRoom)
			return true, false, nil
		}
		minutes := 5
		if len(segments) >= 3 {
			if n := cast.ToInt(segments[2]); n > 0 && n <= 24*60 {
				minutes = n
			}
		}
		database.MutePlayer(target.ID, time.Duration(minutes)*time.Minute)
		database.Broadcast(room.ID, fmt.Sprintf("%s was muted by the owner for %d minute(s).\n", target.Name, minutes))
		_ = target.WriteString(fmt.Sprintf("You were muted by the owner for %d minute(s).\n", minutes))
		return true, false, nil

	case "unmute":
		if len(segments) < 2 {
			_ = player.WriteString("Usage: unmute <id>\n")
			return true, false, nil
		}
		target := database.GetPlayer(cast.ToInt64(segments[1]))
		if target == nil {
			_ = player.WriteError(consts.ErrorsPlayerNotInRoom)
			return true, false, nil
		}
		if database.UnmutePlayer(target.ID) {
			database.Broadcast(room.ID, fmt.Sprintf("%s was unmuted by the owner.\n", target.Name))
		} else {
			_ = player.WriteString(fmt.Sprintf("%s is not muted.\n", target.Name))
		}
		return true, false, nil
	}

	// 房间属性：ds on / ct off / pn 3 ...
	if len(segments) == 3 {
		database.SetRoomProps(room, segments[1], segments[2])
		return true, false, nil
	}
	return false, false, nil
}

func startGame(player *database.Player, room *database.Room) (err error) {
	room.Lock()
	defer room.Unlock()
	// 满员自动开局时每个玩家的协程都会走到这里，先到先开，后来者直接返回
	if room.State != consts.RoomStateWaiting {
		return nil
	}
	switch room.Type {
	default:
		room.Game, err = game.InitGame(room)
	case consts.GameTypeUno:
		room.Game, err = game.InitUnoGame(room)
	case consts.GameTypeRunFast:
		room.Game, err = game.InitRunFastGame(room, rule.RunFastRules)
	case consts.GameTypeMahjong:
		room.Game, err = game.InitMahjongGame(room)
	case consts.GameTypeTexas:
		room.Game, err = texas.Init(room)
	case consts.GameTypeLiar:
		room.Game, err = game.InitLiarGame(room)
	case consts.GameTypeUndercover:
		room.Game, err = game.InitUndercoverGame(room)
	case consts.GameTypeGouji:
		room.Game, err = game.InitGoujiGame(room)
	}
	if err != nil {
		_ = player.WriteError(err)
		return err
	}
	room.State = consts.RoomStateRunning
	database.SyncReady(room) // 开局后清掉 ready 窗口
	return nil
}

func viewRoomPlayers(room *database.Room, currPlayer *database.Player) {
	info := room.Info()
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Room ID: %d\n", info.ID))
	buf.WriteString("Players:\n")
	for playerId := range database.RoomPlayers(info.ID) {
		player := database.GetPlayer(playerId)
		readyFlag := ""
		if ready := database.ReadyStatus(room, playerId); ready.Required {
			if ready.Ready {
				readyFlag = " [ready]"
			} else {
				readyFlag = " [not ready]"
			}
		}
		if info.EnableShowIP {
			buf.WriteString(fmt.Sprintf("%s [%s], score: %d, id: %d, ip: %s%s\n", player.Name, player.Role, player.Amount, player.ID, maskIP(player.IP), readyFlag))
		} else {
			buf.WriteString(fmt.Sprintf("%s [%s], score: %d, id: %d%s\n", player.Name, player.Role, player.Amount, player.ID, readyFlag))
		}
	}

	buf.WriteString("\nSpectators:\n")
	for spectatorId := range database.RoomSpectators(info.ID) {
		spectator := database.GetPlayer(spectatorId)
		if info.EnableShowIP {
			buf.WriteString(fmt.Sprintf("%s [spectator], score: %d, id: %d, ip: %s\n", spectator.Name, spectator.Amount, spectator.ID, maskIP(spectator.IP)))
		} else {
			buf.WriteString(fmt.Sprintf("%s [spectator], score: %d, id: %d\n", spectator.Name, spectator.Amount, spectator.ID))
		}
	}

	buf.WriteString("\nSettings:\n")
	switch info.Type {
	case consts.GameTypeUno:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	case consts.GameTypeMahjong:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "pn:", info.MaxPlayers))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	case consts.GameTypeTexas:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "pn:", info.MaxPlayers))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	case consts.GameTypeLiar:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "jt:", sprintPropsState(info.EnableJokerAsTarget)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	case consts.GameTypeUndercover:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "pn:", info.MaxPlayers))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ucn:", info.UndercoverNum))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "bwm:", sprintPropsState(info.BlankWordMode)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	case consts.GameTypeGouji:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "pn:", info.MaxPlayers))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
	default:
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "lz:", sprintPropsState(info.EnableLaiZi)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v, %-5s%-5v\n", "ds:", sprintPropsState(info.EnableDontShuffle), "sk:", sprintPropsState(info.EnableSkill)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v, %-5s%-5v\n", "pn:", info.MaxPlayers, "ct:", sprintPropsState(info.EnableChat)))
		buf.WriteString(fmt.Sprintf("%-5s%-5v\n", "ip:", sprintPropsState(info.EnableShowIP)))
		pwd := info.Password
		if pwd != "" {
			if info.Creator != currPlayer.ID {
				pwd = "********"
			}
		} else {
			pwd = "off"
		}
		buf.WriteString(fmt.Sprintf("%-5s%-20v\n", "pwd", pwd))
	}
	_ = currPlayer.WriteString(buf.String())
}

func sprintPropsState(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func maskIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	return "*.*.*.*"
}
