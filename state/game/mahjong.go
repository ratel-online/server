package game

import (
	"bytes"
	"fmt"
	"sort"

	mjconsts "github.com/feel-easy/mahjong/consts"
	"github.com/feel-easy/mahjong/event"
	mjgame "github.com/feel-easy/mahjong/game"
	"github.com/feel-easy/mahjong/tile"
	"github.com/feel-easy/mahjong/util"
	"github.com/feel-easy/mahjong/win"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
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
	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[Mahjong.Next] Player %d (Room %d) loop count: %d, room.State: %d\n", player.ID, player.RoomID, loopCount, room.State)
		}
		if room.State == int(consts.StateWaiting) {
			log.Infof("[Mahjong.Next] Player %d exiting, room state changed to waiting, loop count: %d\n", player.ID, loopCount)
			return consts.StateWaiting, nil
		}
		log.Infof("[Mahjong.Next] Player %d waiting for state, loop count: %d\n", player.ID, loopCount)
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
		default:
			return 0, consts.ErrorsChanClosed
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
	room.Game = nil
	room.State = consts.RoomStateWaiting
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
		room.Game = nil
		room.State = consts.RoomStateWaiting
		for _, playerId := range game.PlayerIDs {
			game.States[playerId] <- stateWaiting
		}
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
		loopCount := 0
		for {
			loopCount++
			if loopCount%100 == 0 {
				log.Infof("[handleTake] Player %d (Room %d) finding originally player loop count: %d, current: %d, originally: %d\n", p.ID(), room.ID, loopCount, p.ID(), gameState.OriginallyPlayer.ID())
			}
			if gameState.OriginallyPlayer.ID() == p.ID() {
				log.Infof("[handleTake] Player %d found originally player, loop count: %d\n", p.ID(), loopCount)
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
		room.Game = nil
		room.Banker = p.ID()
		room.State = consts.RoomStateWaiting
		for _, playerId := range game.PlayerIDs {
			game.States[playerId] <- stateWaiting
		}
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
		room.Game = nil
		room.Banker = gameState.CanWin[rand.Intn(len(gameState.CanWin))].ID()
		room.State = consts.RoomStateWaiting
		for _, playerId := range game.PlayerIDs {
			game.States[playerId] <- stateWaiting
		}
		return nil
	}
	if len(gameState.SpecialPrivileges) > 0 {
		pvID := pc.ID()
		flag := false
		for _, i := range []int{mjconsts.PENG, mjconsts.CHI} {
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
		loopCount := 0
		for {
			loopCount++
			if loopCount%100 == 0 {
				log.Infof("[handlePlayMahjong] Player %d (Room %d) finding privilege player loop count: %d, current: %d, target: %d\n", pc.ID(), room.ID, loopCount, pc.ID(), pvID)
			}
			if pc.ID() == pvID {
				log.Infof("[handlePlayMahjong] Player %d found privilege player, loop count: %d\n", pc.ID(), loopCount)
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
	mjPlayers := make([]mjgame.Player, 0, room.Players)
	states := map[int]chan int{}
	roomPlayers := database.RoomPlayers(room.ID)
	for playerId := range roomPlayers {
		player := database.GetPlayer(playerId)
		mjPlayers = append(mjPlayers, database.NewPlayer(player))
		playerIDs = append(playerIDs, int(player.ID))
		states[int(playerId)] = make(chan int, 1)
	}
	mahjong := mjgame.New(mjPlayers)
	mahjong.DealStartingTiles()
	if room.Banker == 0 || !util.IntInSlice(room.Banker, playerIDs) {
		room.Banker = playerIDs[rand.Intn(len(playerIDs))]
	}
	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[InitMahjongGame] Room %d finding banker loop count: %d, current: %d, target: %d\n", room.ID, loopCount, mahjong.Current().ID(), room.Banker)
		}
		if mahjong.Current().ID() == room.Banker {
			log.Infof("[InitMahjongGame] Room %d found banker, loop count: %d\n", room.ID, loopCount)
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
