package game

import (
	"bytes"
	"fmt"
	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/event"
	"github.com/feel-easy/uno/game"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type Uno struct{}

func (g *Uno) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.UnoGame)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf(
		"WELCOME TO %s%s%s!!!\n",
		color.Red.Paint("U"),
		color.Yellow.Paint("N"),
		color.Blue.Paint("O"),
	))
	buf.WriteString(fmt.Sprintf("Your Cards: %s\n", game.Game.GetPlayerCards(int(player.ID))))
	_ = player.WriteString(buf.String())
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[int(player.ID)]
		switch state {
		case stateFirstCard:
			if msg := game.Game.PlayFirstCard(); msg != "" {
				database.Broadcast(room.ID, msg)
			}
			pc := game.Game.Players().Next()
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
	return consts.StateHome
}

func handlePlayUno(room *database.Room, player *database.Player, game *database.UnoGame) error {
	p := game.Game.Current()
	if p.ID() != int(player.ID) {
		game.States[p.ID()] <- statePlay
		return nil
	}
	if !game.HavePlay(player) {
		pc := game.Game.Players().Next()
		game.States[pc.ID()] <- statePlay
	}
	gameState := game.Game.ExtractState(p)
	card, err := p.Play(gameState, game.Game.Deck())
	if err != nil || card == nil {
		event.PlayerPassed.Emit(event.PlayerPassedPayload{
			PlayerName: p.Name(),
		})
		pc := game.Game.Players().Next()
		game.States[pc.ID()] <- statePlay
		return err
	}
	game.Game.Pile().Add(card)
	event.CardPlayed.Emit(event.CardPlayedPayload{
		PlayerName: p.Name(),
		Card:       card,
	})
	if msg := game.Game.PerformCardActions(card); msg != "" {
		database.Broadcast(room.ID, msg)
	}
	if p.NoCards() || game.NeedExit() {
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
	pc := game.Game.Players().Next()
	game.States[pc.ID()] <- statePlay
	return nil
}

func InitUnoGame(room *database.Room) (*database.UnoGame, error) {
	players := make([]int, 0)
	roomPlayers := database.RoomPlayers(room.ID)
	unoPlayers := make([]game.Player, 0)
	states := map[int]chan int{}
	for playerId := range roomPlayers {
		p := database.GetPlayer(playerId)
		players = append(players, int(p.ID))
		unoPlayers = append(unoPlayers, database.NewUnoPlayer(p))
		states[int(playerId)] = make(chan int, 1)
	}
	unoGame := game.New(unoPlayers)
	unoGame.DealStartingCards()
	states[unoGame.Current().ID()] <- stateFirstCard
	return &database.UnoGame{
		Room:    room,
		Players: players,
		States:  states,
		Game:    unoGame,
	}, nil
}
