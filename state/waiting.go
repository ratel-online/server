package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/state/game/texas"
	"strings"
	"time"

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
	access, err := waitingForStart(player, room)
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
		}
	}
	return s.Exit(player), nil
}

func (*waiting) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		isOwner := room.Creator == player.ID
		database.LeaveRoom(room.ID, player.ID)
		database.Broadcast(room.ID, fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, room.Players))
		if isOwner {
			newOwner := database.GetPlayer(room.Creator)
			database.Broadcast(room.ID, fmt.Sprintf("%s become new owner\n", newOwner.Name))
		}
	}
	return consts.StateHome
}

func waitingForStart(player *database.Player, room *database.Room) (bool, error) {
	access := false
	//对局类别
	player.StartTransaction()
	defer player.StopTransaction()
	for {
		signal, err := player.AskForStringWithoutTransaction(time.Second)
		if err != nil && err != consts.ErrorsTimeout {
			return access, err
		}
		if room.State == consts.RoomStateRunning {
			access = true
			break
		}
		signal = strings.ToLower(signal)
		if signal == "ls" || signal == "v" {
			viewRoomPlayers(room, player)
		} else if (signal == "start" || signal == "s") && room.Creator == player.ID && room.Players > 1 {
			//跑得快限制必须三人
			if room.Type == consts.GameTypeRunFast && room.Players != 3 {
				err := player.WriteError(consts.ErrorsGamePlayersInvalid)
				if err != nil {
					return false, err
				}
				continue
			}
			access = true
			room.Lock()
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
			}
			if err != nil {
				room.Unlock()
				_ = player.WriteError(err)
				return access, err
			}
			room.State = consts.RoomStateRunning
			room.Unlock()
			break
		} else if strings.HasPrefix(signal, "set ") && room.Creator == player.ID {
			tags := strings.Split(signal, " ")
			if len(tags) == 3 {
				//跑得快只允许修改房间名和是否开启聊天
				if room.Type == consts.GameTypeRunFast {
					if tags[1] == "pwd" || tags[1] == "ct" {
						database.SetRoomProps(room, tags[1], tags[2])
					}
				} else {
					database.SetRoomProps(room, tags[1], tags[2])
				}
				continue
			}
			if room.EnableChat {
				database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, signal))
			} else {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
			}
		} else if len(signal) > 0 {
			if room.EnableChat {
				database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, signal))
			} else {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
			}
		}
	}
	return access, nil
}

func viewRoomPlayers(room *database.Room, currPlayer *database.Player) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Room ID: %d\n", room.ID))
	buf.WriteString(fmt.Sprintf("%-20s%-10s%-10s\n", "Name", "Score", "Title"))
	for playerId := range database.RoomPlayers(room.ID) {
		title := "player"
		if playerId == room.Creator {
			title = "owner"
		}
		player := database.GetPlayer(playerId)
		buf.WriteString(fmt.Sprintf("%-20s%-10d%-10s\n", player.Name, player.Score, title))
	}
	buf.WriteString("\nSettings:\n")
	switch room.Type {
	case consts.GameTypeUno, consts.GameTypeMahjong:
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
