package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"math/rand"
	"strings"
	"time"
)

type classics struct{}

var (
	classicsStateRob     = 1
	classicsStatePlay    = 2
	classicsStateReset   = 3
	classicsStateWaiting = 4
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

func (s *classics) Next(player *database.Player) (consts.StateID, error) {
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
				log.Error(err)
				return 0, err
			}
		case classicsStateReset:
			if player.ID == room.Creator {
				rand.Seed(time.Now().UnixNano())
				game.States[game.Players[rand.Intn(len(game.States))]] <- classicsStateRob
			}
			return 0, nil
		case classicsStatePlay:
			err := handleClassicsPlay(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case classicsStateWaiting:
			return consts.StateWaiting, nil
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
}

func (*classics) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

func handleClassicsRob(player *database.Player, game *database.Game) error {
	if game.FirstPlayer == player.ID || game.FinalRob {
		if game.FirstRob == 0 {
			err := resetClassicsGame(game)
			if err != nil {
				log.Error(err)
				return err
			}
			database.Broadcast(player.RoomID, "All players have give up the landlord. Game restart.\n")
			for _, playerId := range game.Players {
				game.States[playerId] <- classicsStateReset
			}
		} else if game.FirstRob == game.LastRob || game.FinalRob {
			landlord := database.GetPlayer(game.LastRob)
			if landlord == nil {
				return consts.ErrorsPlayersInvalid
			}
			buf := bytes.Buffer{}
			buf.WriteString(fmt.Sprintf("%s become landlord, and got additional pokers: %s\n", landlord.Name, game.Additional.String()))
			database.Broadcast(player.RoomID, buf.String())
			game.FirstPlayer = landlord.ID
			game.LastPlayer = landlord.ID
			game.Groups[landlord.ID] = 1
			game.Pokers[landlord.ID] = append(game.Pokers[landlord.ID], game.Additional...)
			game.Pokers[landlord.ID].SortByValue()
			game.States[landlord.ID] <- classicsStatePlay
		} else {
			game.States[game.FirstRob] <- classicsStateRob
		}
		return nil
	}
	if game.FirstPlayer == 0 {
		game.FirstPlayer = player.ID
	}
	if player.ID == game.FirstRob {
		game.FinalRob = true
	}
	database.Broadcast(player.RoomID, fmt.Sprintf("Please waiting from %s confirm whether to be a landlord. \n", player.Name))
	_ = player.WriteString("Would you like to be a landlord: (y or n)\n")
	ans, err := player.AskForString(consts.ClassicsRobTimeout)
	if err != nil {
		ans = "n"
	}
	if strings.ToLower(ans) == "y" {
		if game.FirstRob == 0 {
			game.FirstRob = player.ID
		}
		game.LastRob = player.ID
		game.Multiple *= 2
		database.Broadcast(player.RoomID, fmt.Sprintf("%s rob landlord\n", player.Name))
	} else {
		database.Broadcast(player.RoomID, fmt.Sprintf("%s don't rob landlord\n", player.Name))
	}
	game.States[game.NextPlayer(player.ID)] <- classicsStateRob
	return nil
}

func handleClassicsPlay(player *database.Player, game *database.Game) error {
	timeout := consts.ClassicsPlayTimeout
	master := player.ID == game.LastPlayer || game.LastPlayer == 0
	for {
		buf := bytes.Buffer{}
		buf.WriteString("\n")
		if !master && game.LastPokers != nil {
			flag := "landlord"
			if game.IsLandlord(game.LastPlayer) {
				flag = "peasant"
			}
			buf.WriteString(fmt.Sprintf("Last player is %s: %s, sells: %s\n", database.GetPlayer(game.LastPlayer).Name, flag, game.LastPokers.String()))
		}
		buf.WriteString(fmt.Sprintf("Timeout: %ds, it's your turn to play \n", int(timeout.Seconds())))
		buf.WriteString(fmt.Sprintf("Pokers: %s\n", game.Pokers[player.ID].String()))
		_ = player.WriteString(buf.String())
		before := time.Now().Unix()
		pokers := game.Pokers[player.ID]
		ans, err := player.AskForString(timeout)
		if err != nil {
			if master {
				ans = poker.GetAlias(pokers[0].Key)
			} else {
				ans = "p"
			}
		} else {
			timeout -= time.Second * time.Duration(time.Now().Unix()-before)
		}
		ans = strings.ToLower(ans)
		if ans == "" {
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			continue
		} else if ans == "ls" || ans == "v" {
			viewClassicsGame(game, player)
			continue
		} else if ans == "p" || ans == "pass" {
			if master {
				_ = player.WriteString("Have to play! \n")
				continue
			} else {
				nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
				if nextPlayer == nil {
					return consts.ErrorsPlayersInvalid
				}
				database.Broadcast(player.RoomID, fmt.Sprintf("%s passed, next player is %s \n", player.Name, nextPlayer.Name))
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
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			continue
		}
		facesArr := poker.ParseFaces(sells, classicsRules)
		if len(facesArr) == 0 {
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			continue
		}
		lastFaces := game.LastFaces
		if !master && lastFaces != nil {
			access := false
			for _, faces := range facesArr {
				if faces.Compare(*lastFaces) {
					access = true
					lastFaces = &faces
					break
				}
			}
			if !access {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
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
		game.LastPokers = &sells

		if len(pokers) == 0 {
			database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s, win the game! \n", player.Name, sells.String()))
			room := database.GetRoom(player.RoomID)
			if room != nil {
				room.Lock()
				room.Game = nil
				room.State = consts.RoomStateWaiting
				room.Unlock()
			}
			for _, playerId := range game.Players {
				game.States[playerId] <- classicsStateWaiting
			}
			return nil
		}
		nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
		database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s, next player is %s \n", player.Name, sells.String(), nextPlayer.Name))
		game.States[nextPlayer.ID] <- classicsStatePlay
		return nil
	}
}

func initClassicsGame(room *database.Room) (*database.Game, error) {
	distributes := poker.Distribute(room.Players, classicsRules)
	players := make([]int64, 0)
	roomPlayers := database.RoomPlayers(room.ID)
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
		states[players[i]] = make(chan int, 1)
		groups[players[i]] = 0
		pokers[players[i]] = distributes[i]
	}
	rand.Seed(time.Now().UnixNano())
	states[players[rand.Intn(len(states))]] <- classicsStateRob
	return &database.Game{
		States:     states,
		Players:    players,
		Groups:     groups,
		Pokers:     pokers,
		Additional: distributes[len(distributes)-1],
		Multiple:   1,
	}, nil
}

func resetClassicsGame(game *database.Game) error {
	distributes := poker.Distribute(len(game.Players), classicsRules)
	if len(distributes) != len(game.Players)+1 {
		return consts.ErrorsGamePlayersInvalid
	}
	players := game.Players
	for i := range players {
		game.Pokers[players[i]] = distributes[i]
	}
	game.Groups = map[int64]int{}
	game.FirstPlayer = 0
	game.LastPlayer = 0
	game.FirstRob = 0
	game.LastRob = 0
	game.FinalRob = false
	game.Multiple = 1
	return nil
}

func viewClassicsGame(game *database.Game, currPlayer *database.Player) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%s\t\t%s\t\t%s\n", "Name ↓", "Pokers", "Identity"))
	for _, id := range game.Players {
		player := database.GetPlayer(id)
		flag := ""
		if id == currPlayer.ID {
			flag = "*"
		}
		identity := "landlord"
		if game.IsLandlord(id) {
			identity = "peasant"
		}
		buf.WriteString(fmt.Sprintf("%s%s\t\t%d\t\t%s\n", player.Name, flag, len(game.Pokers[id]), identity))
	}
	_ = currPlayer.WriteString(buf.String())
}
