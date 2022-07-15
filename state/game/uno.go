package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/event"
	"github.com/ratel-online/server/uno/game"
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
	buf.WriteString(fmt.Sprintf(
		"WELCOME TO %s%s%s!!!\n",
		color.Red.Paint("U"),
		color.Yellow.Paint("N"),
		color.Blue.Paint("O"),
	))
	buf.WriteString(fmt.Sprintf("Your Cards: %s\n", UnoGame.GetPlayerCards(player.Name)))
	_ = player.WriteString(buf.String())
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		switch state {
		case stateFirstCard:
			if msg := UnoGame.PlayFirstCard(); msg != "" {
				database.Broadcast(room.ID, msg)
			}
			pc := UnoGame.Players().Next()
			game.States[pc.ID()] <- statePlay
		case statePlay:
			err := handlePlayUno(room, player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
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

func handlePlayUno(room *database.Room, player *database.Player, game *database.UnoGame) error {
	p := UnoGame.Current()
	if p.ID() != player.ID {
		game.States[p.ID()] <- statePlay
		return nil
	}
	gameState := UnoGame.ExtractState(p)
	car, _ := p.Play(gameState, UnoGame.Deck())
	if car == nil {
		event.PlayerPassed.Emit(event.PlayerPassedPayload{
			PlayerName: p.Name(),
		})
		pc := UnoGame.Players().Next()
		game.States[pc.ID()] <- statePlay
		return nil
	}
	UnoGame.Pile().Add(car)
	event.CardPlayed.Emit(event.CardPlayedPayload{
		PlayerName: p.Name(),
		Card:       car,
	})
	if msg := UnoGame.PerformCardActions(car); msg != "" {
		database.Broadcast(room.ID, msg)
	}
	if p.NoCards() {
		database.Broadcast(room.ID, fmt.Sprintf("%s wins! \n", p.Name()))
		room.Lock()
		room.Game = nil
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.Players {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	pc := UnoGame.Players().Next()
	game.States[pc.ID()] <- statePlay
	return nil
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
	states[UnoGame.Current().ID()] <- stateFirstCard
	return &database.UnoGame{
		Room:    room,
		Players: players,
		States:  states,
	}, nil
}
