package player

import (
	"math/rand"

	"github.com/ratel-online/server/uno/game"
)

var botNames = []string{
	"Annie", "Braum", "Caitlyn", "Draven",
	"Ezreal", "Fiora", "Graves", "Heimerdinger",
	"Ivern", "Jinx", "Kled", "Lulu",
	"Malphite", "Nunu", "Orianna", "Poppy",
	"Qiyana", "Rakan", "Shaco", "Twisted Fate",
	"Udyr", "Veigar", "Wukong", "Xayah",
	"Yuumi", "Zoe",
}

func CreatePlayers(numberOfPlayers int, humanPlayerName string) []game.Player {
	players := make([]game.Player, 0, numberOfPlayers)
	players = append(players, NewHumanPlayer(humanPlayerName))
	players = append(players, generateBots(numberOfPlayers-1)...)
	return players
}

func generateBots(amount int) []game.Player {
	rand.Shuffle(len(botNames), func(i int, j int) { botNames[i], botNames[j] = botNames[j], botNames[i] })
	bots := make([]game.Player, 0, amount)
	for _, botName := range botNames[:amount] {
		bots = append(bots, NewGoodPlayer(botName))
	}
	return bots
}
