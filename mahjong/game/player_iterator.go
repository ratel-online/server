package game

type PlayerIterator struct {
	players map[int64]*playerController
	cycler  *Cycler
}

func (i *PlayerIterator) GetPlayerController(id int64) *playerController {
	return i.players[id]
}

func newPlayerIterator(players []Player) *PlayerIterator {
	var playerIDs []int64
	playerMap := make(map[int64]*playerController, len(players))
	for _, player := range players {
		playerID := player.PlayerID()
		playerIDs = append(playerIDs, playerID)
		playerMap[playerID] = newPlayerController(player)
	}
	return &PlayerIterator{
		players: playerMap,
		cycler:  NewCycler(playerIDs),
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
