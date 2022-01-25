package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/rule"
	"github.com/ratel-online/server/state/game"
	"strings"
	"time"
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
		return consts.StateGame, nil
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
			access = true
			room.Lock()
			room.Game, err = initGame(room)
			if err != nil {
				_ = player.WriteError(err)
				return access, err
			}
			room.State = consts.RoomStateRunning
			room.Unlock()
			break
		} else if strings.HasPrefix(signal, "set ") {
			tags := strings.Split(signal, " ")
			if len(tags) == 3 {
				room.Properties[tags[1]] = tags[2] == "on"
				continue
			}
			database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, signal))
		} else if len(signal) > 0 {
			database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, signal))
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
	buf.WriteString("Properties: ")
	for k, v := range room.Properties {
		if v {
			buf.WriteString(k + " ")
		}
	}
	buf.WriteString("\n")
	_ = currPlayer.WriteString(buf.String())
}

func initGame(room *database.Room) (*database.Game, error) {
	if room.Type == consts.GameTypeLaiZi {
		room.Properties[consts.RoomPropsLaiZi] = true
	}
	rules := rule.LandlordRules
	if room.Properties[consts.RoomPropsSkill] {
		rules = rule.TeamRules
	}
	return game.InitGame(room, rules)
}
