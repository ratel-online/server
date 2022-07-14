package game

import (
	"fmt"
)

type PlayerIterator struct {
	players map[string]*playerController
	cycler  *Cycler
}

func (i *PlayerIterator) GetPlayerController(name string) *playerController {
	return i.players[name]
}

func newPlayerIterator(players []Player) *PlayerIterator {
	var playerNames []string
	playerMap := make(map[string]*playerController, len(players))
	for _, player := range players {
		playerName := player.NickName()
		playerNames = append(playerNames, playerName)
		playerMap[playerName] = newPlayerController(player)
	}
	return &PlayerIterator{
		players: playerMap,
		cycler:  NewCycler(playerNames),
	}
}

func (i *PlayerIterator) Current() *playerController {
	return i.players[i.cycler.Current()]
}

func (i *PlayerIterator) ForEach(function func(player *playerController)) {
	for range i.players {
		function(i.Current())
		i.Next()
	}
}

func (i *PlayerIterator) Next() *playerController {
	return i.players[i.cycler.Next()]
}

func (i *PlayerIterator) Reverse() {
	i.cycler.Reverse()
	//todo
	fmt.Println("Turn order has been reversed!")
}

func (i *PlayerIterator) Skip() {
	skippedPlayer := i.Next()
	//todo
	fmt.Printf("%s's turn skipped!", skippedPlayer.Name())
}
