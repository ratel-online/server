package texas

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/spf13/cast"
	"strings"
	"time"
)

func bet(player *database.Player, game *database.Texas) error {
	texasPlayer := game.Player(player.ID)
	if game.AllIn == len(game.Players) ||
		(game.MaxBetPlayer != nil && game.MaxBetPlayer.ID == player.ID) {
		return nextRound(game)
	}
	if texasPlayer.Folded || texasPlayer.AllIn {
		return nextPlayer(player, game, stateBet)
	}

	database.Broadcast(player.RoomID, fmt.Sprintf("Next it's %s's turn to bet\n", player.Name), player.ID)

	timeout := consts.BetTimeout
	for {
		before := time.Now().Unix()

		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("Your hand: %s\n", texasPlayer.Hand.TexasString()))
		for _, p := range game.Players {
			status := "betting"
			if p.Folded {
				status = "folded"
			}
			if p.AllIn {
				status = "all in"
			}
			name := p.Name
			if p.ID == player.ID {
				name = "* You"
			}
			buf.WriteString(fmt.Sprintf("%s amount %d, total bets %d, status: %s\n", name, p.Amount, p.Bets, status))
		}
		buf.WriteString("What do you want to do? (call/raise/fold/check/allin)\n")
		_ = player.WriteString(buf.String())
		ans, err := player.AskForString(timeout)
		if err != nil {
			ans = "fold"
		}
		timeout -= time.Second * time.Duration(time.Now().Unix()-before)
		minCall := game.MaxBetAmount - texasPlayer.Bets

		instructions := strings.Split(ans, " ")
		switch instructions[0] {
		case "call":
			if minCall == 0 {
				_ = player.WriteString("You don't need to call, wound you like to check?\n")
				continue
			}
			if texasPlayer.Amount < minCall {
				_ = player.WriteString("You don't have enough money to call\n")
				continue
			}
			texasPlayer.Amount -= minCall
			texasPlayer.Bets += minCall
			game.Pot += minCall
			if game.MaxBetPlayer == nil {
				game.MaxBetPlayer = texasPlayer
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("%s call, bet %d\n", player.Name, minCall))
		case "raise":
			if len(instructions) <= 1 || instructions[1] == "" {
				_ = player.WriteString("Please input the amount you want to raise\n")
				continue
			}
			betAmount, err := cast.ToUintE(instructions[1])
			if err != nil {
				_ = player.WriteString("Invalid amount\n")
				continue
			}
			if betAmount < minCall {
				_ = player.WriteString(fmt.Sprintf("The amount you want to raise is less than the minimum call amount %d\n", minCall))
				continue
			}
			if texasPlayer.Amount < betAmount {
				_ = player.WriteString("You don't have enough money to raise\n")
				continue
			}
			texasPlayer.Amount -= betAmount
			texasPlayer.Bets += betAmount
			game.Pot += betAmount
			game.MaxBetAmount = texasPlayer.Bets
			game.MaxBetPlayer = texasPlayer
			database.Broadcast(player.RoomID, fmt.Sprintf("%s raise, bet %d\n", player.Name, betAmount))
		case "fold":
			texasPlayer.Folded = true
			game.Folded++
			database.Broadcast(player.RoomID, fmt.Sprintf("%s fold\n", player.Name))
			if game.Folded == len(game.Players)-1 {
				return settlementRound(game)
			}
		case "check":
			if texasPlayer.Bets < game.MaxBetAmount {
				_ = player.WriteString("You can't check, because someone else bet higher than you\n")
				continue
			}
			if game.MaxBetPlayer == nil {
				game.MaxBetPlayer = texasPlayer
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("%s check\n", player.Name))
		case "allin":
			betAmount := texasPlayer.Amount
			texasPlayer.AllIn = true
			texasPlayer.Amount = 0
			texasPlayer.Bets += betAmount
			game.Pot += betAmount
			game.AllIn++
			if game.MaxBetPlayer == nil {
				game.MaxBetPlayer = texasPlayer
			}
			if texasPlayer.Bets > game.MaxBetAmount {
				game.MaxBetAmount = texasPlayer.Bets
				game.MaxBetPlayer = texasPlayer
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("%s all in, bet %d\n", player.Name, betAmount))
		default:
			database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, ans))
			continue
		}
		break
	}
	return nextPlayer(player, game, stateBet)
}
