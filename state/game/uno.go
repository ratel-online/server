package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/uno/game"
	"github.com/ratel-online/server/uno/msg"
	"github.com/ratel-online/server/uno/player"
)

type Uno struct {
	players *game.PlayerIterator
	deck    *game.Deck
	pile    *game.Pile
}

func (g *Uno) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game
	unoGame := room.UnoGame
	buf := bytes.Buffer{}
	buf.WriteString(msg.Message.Welcome())
	buf.WriteString(fmt.Sprintf("Your Cards: %s\n", unoGame.GetPlayerCards(player.Name)))
	_ = player.WriteString(buf.String())
	if room.State == consts.RoomStateWaiting {
		return consts.StateWaiting, nil
	}
	state := <-game.States[player.ID]
	switch state {
	case statePlay:
		err := handlePlay(player, game)
		if err != nil {
			log.Error(err)
			return 0, err
		}
	case stateWaiting:
		return consts.StateWaiting, nil
	default:
		return 0, consts.ErrorsChanClosed
	}
	return consts.StateUnoGame, nil
}

func (g *Uno) Exit(player *database.Player) consts.StateID {
	return consts.StateUnoGame
}

func InitUnoGame(room *database.Room) (*database.UnoGame, error) {
	players := make([]int64, 0)
	roomPlayers := database.RoomPlayers(room.ID)
	unoPlayers := make([]game.Player, 0)
	states := map[int64]chan int{}
	for playerId := range roomPlayers {
		p := *database.GetPlayer(playerId)
		players = append(players, p.ID)
		unoPlayers = append(unoPlayers, player.NewHumanPlayer(p.Name))
		states[playerId] = make(chan int, 1)
	}
	rand.Seed(time.Now().UnixNano())
	return &database.UnoGame{
		Room:    room,
		Players: players,
		States:  states,
	}, nil
}
