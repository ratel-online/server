package game

import (
	"bytes"
	"fmt"
	"github.com/feel-easy/mahjong/card"
	mjconsts "github.com/feel-easy/mahjong/consts"
	"github.com/feel-easy/mahjong/event"
	"github.com/feel-easy/mahjong/game"
	"github.com/feel-easy/mahjong/tile"
	"github.com/feel-easy/mahjong/util"
	"github.com/feel-easy/mahjong/win"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"sort"
)

type Mahjong struct{}

func (g *Mahjong) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.Mahjong)
	buf := bytes.Buffer{}
	buf.WriteString("WELCOME TO MAHJONG GAME!!! \n")
	buf.WriteString(fmt.Sprintf("%s is Banker! \n", database.GetPlayer(int64(room.Banker)).Name))
	buf.WriteString(fmt.Sprintf("Your Tiles: %s\n", game.Game.GetPlayerTiles(int(player.ID))))
	_ = player.WriteString(buf.String())
	for {
		if room.State == int(consts.StateWaiting) {
			return consts.StateWaiting, nil
		}
		state := <-game.States[int(player.ID)]
		switch state {
		case statePlay:
			err := handlePlayMahjong(room, player, game)
			if err != nil {
				return 0, err
			}
		case stateTakeCard:
			err := handleTake(room, player, game)
			if err != nil {
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
		return consts.StateHome
	}
	game := room.Game.(*database.Mahjong)
	if game == nil {
		return consts.StateHome
	}
	for _, playerId := range game.PlayerIDs {
		game.States[playerId] <- stateWaiting
	}
	database.Broadcast(player.RoomID, fmt.Sprintf("player %s exit, game over! \n", player.Name))
	database.LeaveRoom(player.RoomID, player.ID)
	room.Lock()
	room.Game = nil
	room.State = consts.RoomStateWaiting
	room.Unlock()
	return consts.StateHome
}

func handleTake(room *database.Room, player *database.Player, game *database.Mahjong) error {
	p := game.Game.Current()
	if p.ID() != int(player.ID) {
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	if game.Game.Deck().NoTiles() {
		database.Broadcast(room.ID, "Game over but no winners!!! \n")
		room.Lock()
		room.Game = nil
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.PlayerIDs {
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
		_, ok, err := p.Take(gameState, game.Game.Deck(), game.Game.Pile())
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
	if p.ID() != int(player.ID) {
		game.States[p.ID()] <- statePlay
		return nil
	}
	gameState := game.Game.ExtractState(p)
	if win.CanWin(p.Hand(), p.GetShowCardTiles()) {
		tiles := p.Tiles()
		sort.Ints(tiles)
		database.Broadcast(room.ID, fmt.Sprintf("%s wins! \n%s \n", p.Name(), tile.ToTileString(tiles)))
		room.Lock()
		room.Game = nil
		room.Banker = p.ID()
		room.State = consts.RoomStateWaiting
		room.Unlock()
		for _, playerId := range game.PlayerIDs {
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
		for _, playerId := range game.PlayerIDs {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	if len(gameState.SpecialPrivileges) > 0 {
		pvID := pc.ID()
		flag := false
		for _, i := range []int{mjconsts.GANG, mjconsts.PENG, mjconsts.CHI} {
			for id, pvs := range gameState.SpecialPrivileges {
				if util.IntInSlice(i, pvs) {
					pvID = id
					flag = true
					break
				}
			}
			if flag {
				break
			}
		}
		for {
			if pc.ID() == pvID {
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
	playerIDs := make([]int, 0, room.Players)
	mjPlayers := make([]game.Player, 0, room.Players)
	states := map[int]chan int{}
	roomPlayers := database.RoomPlayers(room.ID)
	for playerId := range roomPlayers {
		player := database.GetPlayer(playerId)
		mjPlayers = append(mjPlayers, database.NewPlayer(player))
		playerIDs = append(playerIDs, int(player.ID))
		states[int(playerId)] = make(chan int, 1)
	}
	mahjong := game.New(mjPlayers)
	mahjong.DealStartingTiles()
	if room.Banker == 0 || !util.IntInSlice(room.Banker, playerIDs) {
		room.Banker = playerIDs[rand.Intn(len(playerIDs))]
	}
	for {
		if mahjong.Current().ID() == room.Banker {
			break
		}
		mahjong.Next()
	}
	states[mahjong.Current().ID()] <- stateTakeCard
	return &database.Mahjong{
		Room:      room,
		PlayerIDs: playerIDs,
		States:    states,
		Game:      mahjong,
	}, nil
}
