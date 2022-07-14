package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/uno/game"
	"github.com/ratel-online/server/uno/msg"
)

var UnoGame = &game.Game{}

type Uno struct{}

func (g *Uno) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.UnoGame
	buf := bytes.Buffer{}
	buf.WriteString(msg.Message.Welcome())
	buf.WriteString(fmt.Sprintf("Your Cards: %s\n", UnoGame.GetPlayerCards(player.Name)))
	_ = player.WriteString(buf.String())
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		switch state {
		case stateFirstCard:
			UnoGame.PlayFirstCard()
		case statePlay:

		case stateWaiting:
			return consts.StateWaiting, nil
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
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
		unoPlayers = append(unoPlayers, p.GamePlayer())
		states[playerId] = make(chan int, 1)
	}
	rand.Seed(time.Now().UnixNano())
	UnoGame = game.New(unoPlayers)
	UnoGame.DealStartingCards()
	states[players[UnoGame.Current().ID()]] <- stateFirstCard
	return &database.UnoGame{
		Room:    room,
		Players: players,
		States:  states,
	}, nil
}
