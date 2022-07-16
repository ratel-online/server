package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/mahjong/event"
	"github.com/ratel-online/server/mahjong/game"
)

type Mahjong struct{}

func (g *Mahjong) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Mahjong
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprint("WELCOME TO MAHJONG GAME!!!\n"))
	buf.WriteString(fmt.Sprintf("Your Tiles: %s\n", game.Game.GetPlayerTiles(player.Name)))
	_ = player.WriteString(buf.String())
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		switch state {
		case statePlay:
			err := handlePlayMahjong(room, player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case stateWaiting:
			return consts.StateWaiting, nil
		}
	}
}

func (g *Mahjong) Exit(player *database.Player) consts.StateID {
	return consts.StateUnoGame
}

func handlePlayMahjong(room *database.Room, player *database.Player, game *database.Mahjong) error {
	p := game.Game.Current()
	if p.ID() != player.ID {
		game.States[p.ID()] <- statePlay
		return nil
	}
	gameState := game.Game.ExtractState(p)
	tile, win, err := p.Play(gameState, game.Game.Deck())
	if err != nil {
		return err
	}
	if win {
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
	game.Game.Pile().Add(tile)
	event.TilePlayed.Emit(event.TilePlayedPayload{
		PlayerName: p.Name(),
		Tile:       tile,
	})
	pc := game.Game.Players().Next()
	game.States[pc.ID()] <- statePlay
	return nil
}

func InitMahjongGame(room *database.Room) (*database.Mahjong, error) {
	players := make([]int64, 0)
	roomPlayers := database.RoomPlayers(room.ID)
	mjPlayers := make([]game.Player, 0)
	states := map[int64]chan int{}
	for playerId := range roomPlayers {
		p := *database.GetPlayer(playerId)
		players = append(players, p.ID)
		mjPlayers = append(mjPlayers, p.MahjongPlayer())
		states[playerId] = make(chan int, 1)
	}
	rand.Seed(time.Now().UnixNano())
	mahjong := game.New(mjPlayers)
	mahjong.DealStartingTiles()
	tile := mahjong.Deck().DrawOne()
	mahjong.Current().AddTiles([]int{tile})
	states[mahjong.Current().ID()] <- statePlay
	return &database.Mahjong{
		Room:    room,
		Players: players,
		States:  states,
		Game:    mahjong,
	}, nil
}
