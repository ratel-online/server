package texas

import (
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/database"
)

func Init(room *database.Room) (game database.RoomGame, err error) {
	if room.Game != nil {
		return resetGame(room)
	}
	return createGame(room)
}

func createGame(room *database.Room) (database.RoomGame, error) {
	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)

	index := 0
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]*database.TexasPlayer, 0)
	for playerId := range roomPlayers {
		player := database.GetPlayer(playerId)
		players = append(players, &database.TexasPlayer{
			ID:    playerId,
			Name:  player.Name,
			State: make(chan int, 1),
			Hand:  base[index*2 : (index+1)*2],
		})
		index++
	}
	game := &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           0,
		SB:           1,
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
	}
	return game, nextRound(game)
}

func resetGame(room *database.Room) (database.RoomGame, error) {
	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)
	game := room.Game.(*database.Texas)

	texasPlayers := make(map[int64]*database.TexasPlayer)
	for _, texasPlayer := range game.Players {
		texasPlayers[texasPlayer.ID] = texasPlayer
	}

	index := 0
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]*database.TexasPlayer, 0)
	for playerId := range roomPlayers {
		if texasPlayer, ok := texasPlayers[playerId]; ok {
			texasPlayer.Reset()
			texasPlayer.Hand = base[index*2 : (index+1)*2]
			players = append(players, texasPlayer)
		} else {
			player := database.GetPlayer(playerId)
			players = append(players, &database.TexasPlayer{
				ID:    playerId,
				Name:  player.Name,
				State: make(chan int, 1),
				Hand:  base[index*2 : (index+1)*2],
			})
		}
		index++
	}
	newGame := &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           (game.BB + 1) % len(players),
		SB:           (game.BB + 2) % len(players),
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
	}
	return newGame, nextRound(newGame)
}

func nextPlayer(current *database.Player, game *database.Texas, state int) error {
	next := game.NextPlayer(current.ID)
	next.State <- state
	return nil
}
