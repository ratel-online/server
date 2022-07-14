package game

import "github.com/ratel-online/server/uno/ui"

type playerIterator struct {
	players map[string]*playerController
	cycler  *Cycler
}

func newPlayerIterator(players []Player) *playerIterator {
	var playerNames []string
	playerMap := make(map[string]*playerController, len(players))
	for _, player := range players {
		playerName := player.Name()
		playerNames = append(playerNames, playerName)
		playerMap[playerName] = newPlayerController(player)
	}
	return &playerIterator{
		players: playerMap,
		cycler:  NewCycler(playerNames),
	}
}

func (i *playerIterator) Current() *playerController {
	return i.players[i.cycler.Current()]
}

func (i *playerIterator) ForEach(function func(player *playerController)) {
	for range i.players {
		function(i.Current())
		i.Next()
	}
}

func (i *playerIterator) Next() *playerController {
	return i.players[i.cycler.Next()]
}

func (i *playerIterator) Reverse() {
	i.cycler.Reverse()
	ui.Message.TurnOrderReversed()
}

func (i *playerIterator) Skip() {
	skippedPlayer := i.Next()
	ui.Message.PlayerTurnSkipped(skippedPlayer.Name())
}
