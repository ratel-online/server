package texas

import (
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/database"
)

func Init(room *database.Room) (database.RoomGame, error) {
	if room.Game != nil {
		game, err := Reset(room)
		if err != nil {
			return nil, err
		}
		return game, nextRound(game.(*database.Texas))
	}

	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)

	index := 0
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]*database.TexasPlayer, 0)
	bigBlind, smallBlind := 0, 1
	for playerId := range roomPlayers {
		player := database.GetPlayer(playerId)
		players = append(players, &database.TexasPlayer{
			ID:     playerId,
			Name:   player.Name,
			State:  make(chan int, 1),
			Hand:   base[index*2 : (index+1)*2],
			Amount: 10000,
		})
		index++
	}

	game := &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           bigBlind,
		SB:           smallBlind,
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
	}
	return game, nextRound(game)
}

func Reset(room *database.Room) (database.RoomGame, error) {
	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)
	game := room.Game.(*database.Texas)

	players := make([]*database.TexasPlayer, 0)
	index := 0

	roomPlayers := database.RoomPlayers(room.ID)
	for _, texasPlayer := range game.Players {
		if roomPlayers[texasPlayer.ID] {
			texasPlayer.Reset()
			texasPlayer.Hand = base[index*2 : (index+1)*2]
			players = append(players, texasPlayer)
			index++
		}
	}

	return &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           (game.BB + 1) / len(players),
		SB:           (game.SB + 1) / len(players),
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
	}, nil
}

func nextPlayer(current *database.Player, game *database.Texas, state int) error {
	next := game.NextPlayer(current.ID)
	next.State <- state
	return nil
}
