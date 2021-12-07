package state

import (
	"bytes"
	"fmt"
	constx "github.com/ratel-online/core/consts"
	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
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
	classicsStateRob   = 1
	classicsStatePlay  = 2
	classicsStateReset = 3
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
	buf.WriteString(fmt.Sprintf("Your pokers: %s\n", game.Pokers[player.ID].String()))
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		switch state {
		case classicsStateRob:
			err := handleClassicsRob(player, game)
			if err != nil {
				return 0, err
			}
		case classicsStateReset:
			err := player.WriteString("All players have give up the landlord. Game restart.\n")
			if err != nil {
				return 0, player.WriteError(err)
			}
			return 0, nil
		case classicsStatePlay:
			err := handleClassicsPlay(player, game)
			if err != nil {
				return 0, err
			}
		}
	}
}

func (*classics) Exit(player *model.Player) consts.StateID {
	_ = database.LeaveRoom(player.RoomID, player.ID)
	_ = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s exited room!\n", player.Name))
	return consts.StateHome
}

func handleClassicsRob(player *model.Player, game *model.Game) error {
	if game.FirstPlayer == player.ID {
		if game.Landlord == 0 {
			err := resetClassicsGame(game)
			if err != nil {
				log.Error(err)
				return err
			}
			for _, playerId := range game.Players {
				game.States[playerId] <- classicsStateReset
			}
			return nil
		} else {
			landlord := database.GetPlayer(game.Landlord)
			if landlord == nil {
				return consts.ErrorsPlayersInvalid
			}
			buf := bytes.Buffer{}
			buf.WriteString(fmt.Sprintf("%s become landlord, and got additional pokers: %s\n", landlord.Name, game.Additional.String()))
			_ = database.RoomBroadcast(player.RoomID, buf.String())
			game.FirstPlayer = landlord.ID
			game.LastPlayer = landlord.ID
			game.Groups[landlord.ID] = 1
			game.Pokers[landlord.ID] = append(game.Pokers[landlord.ID], game.Additional...)
			game.Pokers[landlord.ID].SortByValue()
			game.States[landlord.ID] <- classicsStatePlay
			return nil
		}
	}
	if game.FirstPlayer == 0 {
		game.FirstPlayer = player.ID
	}
	err := player.WriteString("Would you like to be a landlord y/f\n")
	if err != nil {
		return player.WriteError(err)
	}
	ans, err := player.AskForString(consts.ClassicsRobTimeout)
	if err != nil {
		if err != consts.ErrorsTimeout {
			return err
		}
		ans = "f"
	}
	if strings.ToLower(ans) == "f" {
		game.Landlord = player.ID
		game.Multiple *= 2
	} else {
		game.States[game.NextPlayer(player.ID)] <- classicsStateRob
	}
	return nil
}

func handleClassicsPlay(player *model.Player, game *model.Game) error {
	timeout := consts.ClassicsPlayTimeout
	master := player.ID == game.LastPlayer || game.LastPlayer == 0
	for {
		err := player.WriteString(fmt.Sprintf("Your pokers: %s\n", game.Pokers[player.ID].String()))
		if err != nil {
			return player.WriteError(err)
		}
		before := time.Now().Second()
		ans, err := player.AskForString(timeout)
		if err != nil && err != consts.ErrorsTimeout {
			return err
		}
		pokers := game.Pokers[player.ID]
		if err == consts.ErrorsTimeout {
			if master {
				ans = poker.GetAlias(pokers[0].Key)
			} else {
				ans = "p"
			}
		} else {
			timeout -= time.Second * time.Duration(time.Now().Second()-before)
		}
		if ans == "" {
			err := player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			if err != nil {
				return player.WriteError(err)
			}
			continue
		}
		ans = strings.ToLower(ans)
		if ans == "p" || ans == "pass" {
			if master {
				err := player.WriteString("Have to play! \n")
				if err != nil {
					return player.WriteError(err)
				}
				continue
			} else {
				nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
				if nextPlayer == nil {
					return consts.ErrorsPlayersInvalid
				}
				err := database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s passed, next player is %s \n", player.Name, nextPlayer.Name))
				if err != nil {
					return err
				}
				game.States[nextPlayer.ID] <- classicsStatePlay
				return nil
			}
		}
		currPokers := map[int]modelx.Pokers{}
		for _, v := range pokers {
			currPokers[v.Key] = append(currPokers[v.Key], v)
		}
		sells := make(modelx.Pokers, 0)
		invalid := false
		for _, alias := range ans {
			key := poker.GetKey(string(alias))
			if key == 0 {
				invalid = true
				break
			}
			if len(currPokers[key]) == 0 {
				invalid = true
				break
			}
			if len(currPokers[key]) > 0 {
				sells = append(sells, currPokers[key][len(currPokers[key])-1])
				currPokers[key] = currPokers[key][:len(currPokers[key])-1]
			}
		}
		if invalid {
			err := player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			if err != nil {
				return player.WriteError(err)
			}
			continue
		}
		facesArr := poker.ParseFaces(sells, classicsRules)
		if len(facesArr) == 0 {
			err := player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			if err != nil {
				return player.WriteError(err)
			}
			continue
		}
		lastFaces := game.LastFaces
		if !master && lastFaces != nil {
			access := false
			for _, faces := range facesArr {
				if (faces.Type == constx.FacesBomb || lastFaces.Type == faces.Type) && faces.Score > lastFaces.Score {
					access = true
					lastFaces = &faces
					break
				}
			}
			if !access {
				err := player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
				if err != nil {
					return player.WriteError(err)
				}
				continue
			}
		} else {
			lastFaces = &facesArr[0]
		}
		pokers = make(modelx.Pokers, 0)
		for _, curr := range currPokers {
			pokers = append(pokers, curr...)
		}
		pokers.SortByValue()
		game.Pokers[player.ID] = pokers
		game.LastPlayer = player.ID
		game.LastFaces = lastFaces

		if len(pokers) == 0 {
			err = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s played %s, win the game! \n", player.Name, sells.String()))
			if err != nil {
				return err
			}
			err := resetClassicsGame(game)
			if err != nil {
				log.Error(err)
				return err
			}
			for _, playerId := range game.Players {
				game.States[playerId] <- classicsStateReset
			}
			return nil
		}
		nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
		if nextPlayer == nil {
			return consts.ErrorsPlayersInvalid
		}
		err = database.RoomBroadcast(player.RoomID, fmt.Sprintf("%s played %s, next player is %s \n", player.Name, sells.String(), nextPlayer.Name))
		if err != nil {
			return err
		}
		game.States[nextPlayer.ID] <- classicsStatePlay
		return nil
	}
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
