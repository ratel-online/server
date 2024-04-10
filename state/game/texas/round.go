package texas

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func nextRound(game *database.Texas) error {
	switch game.Round {
	case "start":
		return preFlopRound(game)
	case "per-flop":
		return flopRound(game)
	case "flop":
		return turnRound(game)
	case "turn":
		return riverRound(game)
	case "river":
		return settlementRound(game)
	default:
		return consts.ErrorsUnknownTexasRound
	}
}

func preFlopRound(game *database.Texas) error {
	game.Round = "per-flop"
	game.MaxBetPlayer = nil
	game.BBPlayer().Amount -= 20
	game.BBPlayer().Bets = 20
	game.SBPlayer().Amount -= 10
	game.SBPlayer().Bets = 10

	for id := range database.RoomPlayers(game.Room.ID) {
		player := database.GetPlayer(id)
		texasPlayer := game.Player(id)

		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("Game starting!\n"))
		if game.SBPlayer().ID != player.ID {
			buf.WriteString(fmt.Sprintf("Your hand: %s\n", texasPlayer.Hand.TexasString()))
		}
		if game.BBPlayer().ID == player.ID {
			buf.WriteString("You are big blind, bet 20 automatically.\n")
		} else {
			buf.WriteString(fmt.Sprintf("Big blind: %s, Bet 20\n", game.Players[game.BB].Name))
		}
		if game.SBPlayer().ID == player.ID {
			buf.WriteString("You are small blind, bet 10 automatically.\n")
		} else {
			buf.WriteString(fmt.Sprintf("Small blind: %s, Bet 10\n", game.Players[game.SB].Name))
			buf.WriteString(fmt.Sprintf("Pre-flop round, please wait for small blind %s to bet\n", game.Players[game.SB].Name))
		}
		_ = player.WriteString(buf.String())
	}
	game.SBPlayer().State <- stateBet
	return nil
}

func flopRound(game *database.Texas) error {
	game.Round = "flop"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:4]...)
	game.Pool = game.Pool[4:]
	database.Broadcast(game.Room.ID, fmt.Sprintf("Flop round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func turnRound(game *database.Texas) error {
	game.Round = "turn"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:2]...)
	game.Pool = game.Pool[2:]
	database.Broadcast(game.Room.ID, fmt.Sprintf("Turn round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func riverRound(game *database.Texas) error {
	game.Round = "river"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:2]...)
	game.Pool = game.Pool[2:]
	database.Broadcast(game.Room.ID, fmt.Sprintf("River round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func settlementRound(game *database.Texas) error {
	buf := bytes.Buffer{}
	buf.WriteString("Settlement round\n")
	buf.WriteString(fmt.Sprintf("Board: %s\n", game.Board.TexasString()))

	if game.Folded == len(game.Players)-1 {
		var winner *database.TexasPlayer
		for _, player := range game.Players {
			if !player.Folded {
				winner = player
				break
			}
		}
		if winner != nil {
			winner.Amount += game.Pot
			buf.WriteString(fmt.Sprintf("Winner: %s, got all pot: %d\n", winner.Name, game.Pot))
		} else {
			buf.WriteString("All players folded\n")
		}
	} else {
		buf.WriteString("Players' hands:\n")
		var maxFaces *model.TexasFaces
		var maxPlayers []int64
		for _, player := range game.Players {
			if player.Folded {
				continue
			}
			faces, err := poker.ParseTexasFaces(player.Hand, game.Board)
			if err != nil {
				return err
			}
			buf.WriteString(fmt.Sprintf("%s: %s, type: %s, score: %d\n", player.Name, player.Hand.TexasString(), faces.Type, faces.Score))
			if maxFaces == nil || (maxFaces.Type < faces.Type || maxFaces.Score < faces.Score) {
				maxFaces = faces
				maxPlayers = []int64{player.ID}
				continue
			}
			if maxFaces.Type == faces.Type && maxFaces.Score == faces.Score {
				maxPlayers = append(maxPlayers, player.ID)
			}
		}
		winners := make([]*database.TexasPlayer, 0)
		for _, id := range maxPlayers {
			winners = append(winners, game.Player(id))
		}
		if len(winners) == 1 {
			buf.WriteString(fmt.Sprintf("Winner: %s, got all pot: %d\n", winners[0].Name, game.Pot))
		} else {
			buf.WriteString("Winners: ")
			for i, winner := range winners {
				if i != 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(winner.Name)
			}
			buf.WriteString(fmt.Sprintf(", half all pot: %d\n", game.Pot))
		}
		for _, winner := range winners {
			winner.Amount += game.Pot / uint(len(winners))
		}
	}
	buf.WriteString("Please room creator to start a new game\n")
	database.Broadcast(game.Room.ID, buf.String())

	room := game.Room
	room.State = consts.RoomStateWaiting
	for _, player := range game.Players {
		player.State <- stateWaiting
	}
	return nil
}
