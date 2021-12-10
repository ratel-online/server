package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state/classics"
	"github.com/ratel-online/server/state/laizi"
	"strings"
	"time"
)

type waiting struct{}

func (s *waiting) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, consts.ErrorsExist
	}
	access := false
	for {
		signal, err := player.AskForString(time.Second)
		if err != nil && err != consts.ErrorsTimeout {
			return 0, err
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
				return 0, err
			}
			room.State = consts.RoomStateRunning
			room.Unlock()
			break
		}
	}
	if access {
		if room.Type == consts.GameTypeClassic {
			return consts.StateClassics, nil
		} else if room.Type == consts.GameTypeLaiZi {
			return consts.StateLaiZi, nil
		}
	}
	return s.Exit(player), nil
}

func (*waiting) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		isOwner := room.Creator == player.ID
		database.LeaveRoom(player.RoomID, player.ID)
		database.Broadcast(room.ID, fmt.Sprintf("%s exited room! room current has %d players\n", player.Name, room.Players))
		if isOwner {
			newOwner := database.GetPlayer(room.Creator)
			database.Broadcast(room.ID, fmt.Sprintf("%s become new owner\n", newOwner.Name))
		}
	}
	return consts.StateHome
}

func viewRoomPlayers(room *database.Room, currPlayer *database.Player) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-20s%-10s%-10s\n", "Name", "Score", "Title"))
	for playerId := range database.RoomPlayers(room.ID) {
		title := "player"
		if playerId == room.Creator {
			title = "owner"
		}
		player := database.GetPlayer(playerId)
		buf.WriteString(fmt.Sprintf("%-20s%-10d%-10s\n", player.Name, player.Score, title))
	}
	_ = currPlayer.WriteString(buf.String())
}

func initGame(room *database.Room) (*database.Game, error) {
	if room.Type == consts.GameTypeClassic {
		return classics.InitGame(room)
	} else if room.Type == consts.GameTypeLaiZi {
		return laizi.InitGame(room)
	}
	return nil, nil
}
