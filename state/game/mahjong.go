package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/mahjong/card"
	mjconsts "github.com/ratel-online/server/mahjong/consts"
	"github.com/ratel-online/server/mahjong/event"
	"github.com/ratel-online/server/mahjong/game"
	"github.com/ratel-online/server/mahjong/tile"
	cwin "github.com/ratel-online/server/mahjong/win"
)

type Mahjong struct{}

func (g *Mahjong) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Mahjong
	buf := bytes.Buffer{}
	buf.WriteString("WELCOME TO MAHJONG GAME!!! \n")
	buf.WriteString(fmt.Sprintf("%s is Banker! \n", database.GetPlayer(room.Banker).Name))
	buf.WriteString(fmt.Sprintf("Your Tiles: %s\n", game.Game.GetPlayerTiles(player.ID)))
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
				if err, ok := err.(consts.Error); ok {
					if err == consts.ErrorsExist {
						player.WriteString("Don't quit a good game！\n")
						game.States[player.ID] <- statePlay
						log.Error(err)
						continue
					}
				}
				return 0, err
			}
		case stateTakeCard:
			err := handleTakeMahjong(room, player, game)
			if err != nil {
				if _, ok := err.(consts.Error); ok {
					if err == consts.ErrorsExist {
						player.WriteString("Don't quit a good game！\n")
						game.States[player.ID] <- stateTakeCard
						log.Error(err)
						continue
					}
				}
				return 0, err
			}
		case stateWaiting:
			return consts.StateWaiting, nil
		}
	}
}

func (g *Mahjong) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return consts.StateMahjong
	}
	game := room.Mahjong
	if game == nil {
		return consts.StateMahjong
	}
	for _, playerId := range game.Players {
		game.States[playerId] <- stateWaiting
	}
	database.Broadcast(player.RoomID, fmt.Sprintf("player %s exit, game over! \n", player.Name))
	database.LeaveRoom(player.RoomID, player.ID)
	room.Lock()
	room.Game = nil
	room.State = consts.RoomStateWaiting
	room.Unlock()
	return consts.StateMahjong
}

func handleTakeMahjong(room *database.Room, player *database.Player, game *database.Mahjong) error {
	p := game.Game.Current()
	if p.ID() != player.ID {
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	if game.Game.Deck().NoTiles() {
		database.Broadcast(room.ID, "Game over but no winners!!! \n")
		room.Lock()
		room.Game = nil
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.Players {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	if t, ok := card.HaveGang(p.Hand()); ok {
		p.DarkGang(t)
		p.TryBottomDecking(game.Game.Deck())
		game.States[p.ID()] <- statePlay
		return nil
	}
	if card.CanGang(p.GetShowCardTiles(), p.LastTile()) {
		showCard := p.FindShowCard(p.LastTile())
		showCard.ModifyPongToKong(mjconsts.GANG, false)
		p.TryBottomDecking(game.Game.Deck())
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	gameState := game.Game.ExtractState(p)
	if len(gameState.SpecialPrivileges) > 0 {
		_, ok, err := p.TakeMahjong(gameState, game.Game.Deck(), game.Game.Pile())
		if err != nil {
			return err
		}
		if ok {
			game.States[p.ID()] <- statePlay
			return nil
		}
		for {
			if gameState.OriginallyPlayer.ID() == p.ID() {
				p.TryTopDecking(game.Game.Deck())
				game.States[p.ID()] <- statePlay
				return nil
			}
			p = game.Game.Next()
		}
	}
	p.TryTopDecking(game.Game.Deck())
	game.States[p.ID()] <- statePlay
	return nil
}

func handlePlayMahjong(room *database.Room, player *database.Player, game *database.Mahjong) error {
	p := game.Game.Current()
	if p.ID() != player.ID {
		game.States[p.ID()] <- statePlay
		return nil
	}
	gameState := game.Game.ExtractState(p)
	if cwin.CanWin(p.Hand(), p.GetShowCardTiles()) {
		tiles := p.Tiles()
		sort.Ints(tiles)
		database.Broadcast(room.ID, fmt.Sprintf("%s wins! \n%s \n", p.Name(), tile.ToTileString(tiles)))
		room.Lock()
		room.Game = nil
		room.Banker = p.ID()
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.Players {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	if _, ok := card.HaveGang(p.Hand()); ok {
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	if card.CanGang(p.GetShowCardTiles(), p.LastTile()) {
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	til, err := p.Play(gameState)
	if err != nil {
		return err
	}
	game.Game.Pile().Add(til)
	game.Game.Pile().SetLastPlayer(p)
	event.TilePlayed.Emit(event.TilePlayedPayload{
		PlayerName: p.Name(),
		Tile:       til,
	})
	pc := game.Game.Next()
	game.Game.Pile().SetOriginallyPlayer(pc)
	gameState = game.Game.ExtractState(p)
	if len(gameState.CanWin) > 0 {
		for _, p := range gameState.CanWin {
			tiles := append(p.Tiles(), gameState.LastPlayedTile)
			sort.Ints(tiles)
			database.Broadcast(room.ID, fmt.Sprintf("%s wins! \n%s \n", p.Name(), tile.ToTileString(tiles)))
		}
		room.Lock()
		room.Game = nil
		room.Banker = gameState.CanWin[rand.Intn(len(gameState.CanWin))].ID()
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.Players {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	if len(gameState.SpecialPrivileges) > 0 {
		for {
			if _, ok := gameState.SpecialPrivileges[pc.ID()]; ok {
				game.States[pc.ID()] <- stateTakeCard
				return nil
			}
			pc = game.Game.Next()
		}
	}
	game.States[pc.ID()] <- stateTakeCard
	return nil
}

func InitMahjongGame(room *database.Room) (*database.Mahjong, error) {
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]int64, 0, len(roomPlayers))
	mjPlayers := make([]game.Player, 0, len(roomPlayers))
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
	if room.Banker == 0 {
		room.Banker = players[rand.Intn(len(players))]
	}
	for {
		if mahjong.Current().ID() == room.Banker {
			break
		}
		mahjong.Next()
	}
	states[mahjong.Current().ID()] <- stateTakeCard
	return &database.Mahjong{
		Room:    room,
		Players: players,
		States:  states,
		Game:    mahjong,
	}, nil
}
