package game

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/feel-easy/mahjong/card"
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
	// Check if there's a dark gang (暗杠)
	if t, ok := card.HaveGang(p.Hand()); ok {
		_ = player.WriteString(fmt.Sprintf("You can 暗杠 %s, do it? (y/n)\n", tile.Tile(t)))
		ans, err := player.AskForString(consts.PlayMahjongTimeout)
		if err == nil && (ans == "y" || ans == "Y") {
			p.DarkGang(t)
			p.TryBottomDecking(game.Game.Deck())
			game.States[p.ID()] <- statePlay
			return nil
		}
		// choose not to dark gang, continue normal flow
	}
	// Check if there's a pong that can be upgraded to gang (加杠)
	// Only pong can be upgraded, not chi (sequence)
	var pongToGang *mjgame.ShowCard
	for _, sc := range p.GetShowCard() {
		if sc.IsPeng() {
			pongTile := sc.GetTile()
			// Count how many of this tile are in hand
			count := 0
			for _, handTile := range p.Hand() {
				if handTile == pongTile {
					count++
				}
			}
			// If exactly 1 tile in hand, we can add kong
			if count == 1 {
				pongToGang = sc
				break
			}
		}
	}
	if pongToGang != nil {
		_ = player.WriteString(fmt.Sprintf("You can 加杠 %s, do it? (y/n)\n", tile.Tile(pongToGang.GetTile())))
		ans, err := player.AskForString(consts.PlayMahjongTimeout)
		if err == nil && (ans == "y" || ans == "Y") {
			pongToGang.ModifyPongToKong(mjconsts.GANG, false)
			p.TryBottomDecking(game.Game.Deck())
			game.States[p.ID()] <- stateTakeCard
			return nil
		}
		// choose not to add kong, continue normal flow
	}
	gameState := game.Game.ExtractState(p)
	if len(gameState.SpecialPrivileges) > 0 {
		op, ok, err := p.Take(gameState, game.Game.Deck(), game.Game.Pile())
		if err != nil {
			return err
		}
		if ok {
			// Player successfully performed chi/peng/gang
			// Now find who should play next
			// Only the originally triggered player (who drew the winning tile) gets priority

			// For CHI: next player plays immediately (after discarding from draw)
			// For PENG/GANG: if not the originally player, skip to next
			//                if is the originally player, they get turn to play

			loopCount := 0
			maxIterations := len(game.PlayerIDs) + 1
			for {
				loopCount++
				if loopCount%100 == 0 {
					log.Infof("[handleTake] Player %d (Room %d) finding play turn loop count: %d, current: %d, originally: %d, op: %d\n", p.ID(), room.ID, loopCount, p.ID(), gameState.OriginallyPlayer.ID(), op)
				}
				if loopCount > maxIterations {
					log.Errorf("[handleTake] Player %d exceeded max iterations, defaulting to originally player\n", p.ID())
					game.States[gameState.OriginallyPlayer.ID()] <- statePlay
					return nil
				}

				// CHI: current player always plays after chi
				if op == mjconsts.CHI {
					game.States[p.ID()] <- statePlay
					return nil
				}

				// PENG/GANG: only originally player plays
				if p.ID() == gameState.OriginallyPlayer.ID() {
					game.States[p.ID()] <- statePlay
					return nil
				}

				p = game.Game.Next()
			}
		}
		// Player chose not to operate, continue to next player
		loopCount := 0
		maxIterations := len(game.PlayerIDs) + 1
		for {
			loopCount++
			if loopCount%100 == 0 {
				log.Infof("[handleTake] Player %d (Room %d) finding next taker loop count: %d, current: %d, originally: %d\n", p.ID(), room.ID, loopCount, p.ID(), gameState.OriginallyPlayer.ID())
			}
			if loopCount > maxIterations {
				log.Errorf("[handleTake] Player %d exceeded max iterations looking for next player\n", p.ID())
				// Fallback: give turn to originally player
				game.States[gameState.OriginallyPlayer.ID()] <- stateTakeCard
				return nil
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
	if _, ok := card.HaveGang(p.Hand()); ok {
		game.States[p.ID()] <- stateTakeCard
		return nil
	}
	// Check if there's a pong that can be upgraded to gang
	for _, sc := range p.GetShowCard() {
		if sc.IsPeng() {
			pongTile := sc.GetTile()
			// Count how many of this tile are in hand
			count := 0
			for _, handTile := range p.Hand() {
				if handTile == pongTile {
					count++
				}
			}
			// If exactly 1 tile in hand, we can add kong
			if count == 1 {
				game.States[p.ID()] <- stateTakeCard
				return nil
			}
		}
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
		loopCount := 0
		maxIterations := len(game.PlayerIDs) + 1
		for {
			loopCount++
			if loopCount%100 == 0 {
				log.Infof("[handlePlayMahjong] Player %d (Room %d) finding privilege player loop count: %d, current: %d, target: %d\n", pc.ID(), room.ID, loopCount, pc.ID(), pvID)
			}
			if loopCount > maxIterations {
				log.Errorf("[handlePlayMahjong] Player %d exceeded max iterations looking for privilege player with ID %d\n", pc.ID(), pvID)
				// Fallback: give turn to the privilege player directly
				game.States[pvID] <- stateTakeCard
				return nil
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
	maxIterations := len(playerIDs) + 1
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[InitMahjongGame] Room %d finding banker loop count: %d, current: %d, target: %d\n", room.ID, loopCount, mahjong.Current().ID(), room.Banker)
		}
		if loopCount > maxIterations {
			log.Errorf("[InitMahjongGame] Room %d exceeded max iterations looking for banker %d\n", room.ID, room.Banker)
			// Banker not found, use current player as banker
			room.Banker = mahjong.Current().ID()
			break
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
