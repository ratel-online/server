package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/arrays"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
	"math/rand"
	"strings"
	"time"
)

type classics struct{}

var (
	classicsStateRob  = 1
	classicsStatePlay = 1
)

var classicsRules = _classicsRules{}

type _classicsRules struct {
}

func (c _classicsRules) Value(key int) int {
	if key == 1 {
		return 12
	} else if key == 2 {
		return 13
	} else if key > 13 {
		return key
	}
	return key - 2
}

func (c _classicsRules) IsStraight(faces []int, count int) bool {
	if faces[len(faces)-1]-faces[0] != len(faces)-1 {
		return false
	}
	if faces[len(faces)-1] > 12 {
		return false
	}
	if count == 1 {
		return len(faces) >= 5
	} else if count == 2 {
		return len(faces) >= 3
	} else if count > 2 {
		return len(faces) >= 2
	}
	return false
}

func (c _classicsRules) StraightBoundary() (int, int) {
	return 1, 12
}

func (c _classicsRules) Reserved() bool {
	return true
}

func (s *classics) Next(player *model.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game

	buf := bytes.Buffer{}
	buf.WriteString("Game starting!\n")
	buf.WriteString("Your pokers: " + game.Pokers[player.ID].String() + "\n")
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	for {
		state := <-game.States[player.ID]
		switch state {
		case classicsStateRob:
			if game.FirstPlayer == player.ID {
				if game.Landlord == 0 {
					err = resetClassicsGame(game)
					if err != nil {
						log.Error(err)
						return s.Exit(player), nil
					}
					return consts.StateClassics, nil
				} else {
					landlord := database.GetPlayer(game.Landlord)
					if landlord == nil {
						return s.Exit(player), nil
					}
					buf := bytes.Buffer{}
					buf.WriteString(fmt.Sprintf("%s become landlord, and got additional pokers: %s\n", landlord.Name, game.Additional.String()))
					_ = database.RoomBroadcast(room.ID, buf.String())
					game.Groups[landlord.ID] = 1
					game.Pokers[landlord.ID] = append(game.Pokers[landlord.ID], game.Additional...)
					game.Pokers[landlord.ID].SortByValue()
					game.States[landlord.ID] <- classicsStatePlay
					continue
				}
			}
			if game.FirstPlayer == 0 {
				game.FirstPlayer = player.ID
			}
			err := player.WriteString("Would you like to be a landlord y/f\n")
			if err != nil {
				return 0, player.WriteError(err)
			}
			ans, err := player.AskForString(consts.ClassicsRobTimeout)
			if err != nil {
				if err != consts.ErrorsTimeout {
					return 0, err
				}
				ans = "f"
			}
			if strings.ToLower(ans) == "f" {
				game.Landlord = player.ID
				game.Multiple *= 2
			} else {
				curr := arrays.IndexOf(game.Players, player.ID)
				next := (curr + 1) % len(game.Pokers)
				game.States[game.Players[next]] <- classicsStateRob
			}
		}
	}
	return 0, nil
}

func (*classics) Exit(player *model.Player) consts.StateID {
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s exited room!\n", player.Name))
	return consts.StateHome
}

func initClassicsGame(room *model.Room) (*model.Game, error) {
	distributes := poker.Distribute(room.Players, classicsRules)
	players := make([]int64, 0)
	roomPlayers := database.GetRoomPlayers(room.ID)
	for playerId := range roomPlayers {
		players = append(players, playerId)
	}
	if len(distributes) != len(players)+1 {
		return nil, consts.ErrorsGamePlayersInvalid
	}
	states := map[int64]chan int{}
	groups := map[int64]int{}
	pokers := map[int64]modelx.Pokers{}
	for i := range players {
		states[players[i]] = make(chan int)
		groups[players[i]] = 0
		pokers[players[i]] = distributes[i]
	}
	rand.Seed(time.Now().UnixNano())
	states[players[rand.Intn(len(states))]] <- classicsStateRob
	return &model.Game{
		States:     states,
		Players:    players,
		Groups:     groups,
		Pokers:     pokers,
		Additional: distributes[len(distributes)-1],
		Multiple:   1,
	}, nil
}

func resetClassicsGame(game *model.Game) error {
	distributes := poker.Distribute(len(game.Players), classicsRules)
	if len(distributes) != len(game.Players)+1 {
		return consts.ErrorsGamePlayersInvalid
	}
	players := game.Players
	for i := range players {
		game.Pokers[players[i]] = distributes[i]
	}
	rand.Seed(time.Now().UnixNano())
	game.States[players[rand.Intn(len(states))]] <- classicsStateRob
	game.Groups = map[int64]int{}
	game.Multiple = 1
	game.Landlord = 0
	return nil
}
