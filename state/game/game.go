package game

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/rule"
	"strconv"
	"strings"
	"time"

	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/skill"
)

type Game struct{}

var (
	stateRob       = 1
	statePlay      = 2
	stateReset     = 3
	stateWaiting   = 4
	stateFirstCard = 5
	stateTakeCard  = 6
)

func (g *Game) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.Game)
	buf := bytes.Buffer{}
	if game.Room.EnableLaiZi {
		if game.Room.EnableSkill {
			game.Pokers[player.ID].SetOaa(game.Universals...)
			buf.WriteString(fmt.Sprintf("Game starting! Universals: %s %s\n", poker.GetDesc(game.Universals[0]), poker.GetDesc(game.Universals[1])))
		} else {
			game.Pokers[player.ID].SetOaa(game.Universals[0])
			buf.WriteString(fmt.Sprintf("Game starting! First universal: %s\n", poker.GetDesc(game.Universals[0])))
		}
		game.Pokers[player.ID].SortByOaaValue()
	} else {
		buf.WriteString(fmt.Sprintf("Game starting!\n"))
	}
	if game.Room.EnableSkill {
		buf.WriteString(fmt.Sprintf("Got skill %s\n", skill.Skills[consts.SkillID(game.Skills[player.ID])].Name()))
	}
	buf.WriteString(fmt.Sprintf("Your pokers: %s\n", game.Pokers[player.ID].String()))
	_ = player.WriteString(buf.String())
	for {
		if room.State == consts.RoomStateWaiting {
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		switch state {
		case stateRob:
			if !game.Room.EnableLandlord {
				// reset all players group
				for i, id := range game.Players {
					game.Groups[id] = i
					if game.Room.EnableLaiZi {
						game.Pokers[id].SetOaa(game.Universals...)
						game.Pokers[id].SortByOaaValue()
					}
				}
				game.States[player.ID] <- statePlay
			} else {
				err := handleRob(player, game)
				if err != nil {
					log.Error(err)
					return 0, err
				}
			}
		case stateReset:
			if player.ID == room.Creator {
				game.States[game.Players[rand.Intn(len(game.States))]] <- stateRob
			}
			return 0, nil
		case statePlay:
			err := handlePlay(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case stateWaiting:
			return consts.StateWaiting, nil
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
}

func (*Game) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

func handleRob(player *database.Player, game *database.Game) error {
	if game.FirstPlayer == player.ID && !game.FinalRob {
		if game.FirstRob == 0 {
			err := resetGame(game)
			if err != nil {
				log.Error(err)
				return err
			}
			database.Broadcast(player.RoomID, "All players have give up the landlord, restarting...\n")
			for _, playerId := range game.Players {
				game.States[playerId] <- stateReset
			}
		} else if game.FirstRob == game.LastRob {
			landlord := database.GetPlayer(game.LastRob)
			game.FirstPlayer = landlord.ID
			game.LastPlayer = landlord.ID
			game.Groups[landlord.ID] = 1
			game.Pokers[landlord.ID] = append(game.Pokers[landlord.ID], game.Additional...)
			game.Pokers[landlord.ID].SortByOaaValue()

			buf := bytes.Buffer{}
			if game.Room.EnableLaiZi {
				buf.WriteString(fmt.Sprintf("%s became landlord, got pokers: %s, last universal: %s\n", landlord.Name, game.Additional.String(), poker.GetDesc(game.Universals[1])))
				for _, pokers := range game.Pokers {
					pokers.SetOaa(game.Universals...)
					pokers.SortByOaaValue()
				}
			} else {
				buf.WriteString(fmt.Sprintf("%s became landlord, got pokers: %s\n", landlord.Name, game.Additional.String()))
			}
			database.Broadcast(player.RoomID, buf.String())
			game.States[landlord.ID] <- statePlay
		} else {
			game.FinalRob = true
			game.States[game.FirstRob] <- stateRob
		}
		return nil
	}
	if game.FirstPlayer == 0 {
		game.FirstPlayer = player.ID
		database.Broadcast(player.RoomID, fmt.Sprintf("%s's turn to rob\n", player.Name), player.ID)
	}

	timeout := consts.RobTimeout
	for {
		before := time.Now().Unix()
		_ = player.WriteString("Are you want to become landlord? (y or n)\n")
		ans, err := player.AskForString(timeout)
		if err != nil && err != consts.ErrorsExist {
			ans = "n"
		}
		timeout -= time.Second * time.Duration(time.Now().Unix()-before)
		ans = strings.ToLower(ans)
		if ans == "y" {
			if game.FirstRob == 0 {
				game.FirstRob = player.ID
			}
			game.LastRob = player.ID
			game.Multiple *= 2
			database.Broadcast(player.RoomID, fmt.Sprintf("%s rob\n", player.Name))
			break
		} else if ans == "n" {
			database.Broadcast(player.RoomID, fmt.Sprintf("%s don't rob\n", player.Name))
			break
		} else {
			_ = player.WriteError(consts.ErrorsInputInvalid)
			continue
		}
	}
	if game.FinalRob {
		game.FinalRob = false
		game.FirstRob = game.LastRob
		game.States[game.FirstPlayer] <- stateRob
	} else {
		game.States[game.NextPlayer(player.ID)] <- stateRob
	}
	return nil
}

func playing(player *database.Player, game *database.Game, master bool, playTimes int) error {
	timeout := game.PlayTimeOut[player.ID]
	for {
		buf := bytes.Buffer{}
		buf.WriteString("\n")
		if !master && len(game.LastPokers) > 0 {
			buf.WriteString(fmt.Sprintf("Last player: %s (%s), played: %s\n", database.GetPlayer(game.LastPlayer).Name, game.Team(game.LastPlayer), game.LastPokers.String()))
		}
		buf.WriteString(fmt.Sprintf("Timeout: %ds, pokers: %s\n", int(timeout.Seconds()), game.Pokers[player.ID].String()))
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
			viewGame(game, player)
			continue
		} else if ans == "p" || ans == "pass" {
			if master {
				_ = player.WriteError(consts.ErrorsHaveToPlay)
				continue
			} else {
				nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
				database.Broadcast(player.RoomID, fmt.Sprintf("%s passed, next %s\n", player.Name, nextPlayer.Name))
				game.States[nextPlayer.ID] <- statePlay
				return nil
			}
		}
		normalPokers := map[int]modelx.Pokers{}
		universalPokers := make(modelx.Pokers, 0)
		realSellKeys := make([]int, 0)
		for _, v := range pokers {
			if v.Oaa {
				universalPokers = append(universalPokers, v)
			} else {
				normalPokers[v.Key] = append(normalPokers[v.Key], v)
			}
		}
		sells := make(modelx.Pokers, 0)
		invalid := false
		for _, alias := range ans {
			key := poker.GetKey(string(alias))
			if key == 0 {
				invalid = true
				break
			}
			if len(normalPokers[key]) == 0 {
				if key == 14 || key == 15 || len(universalPokers) == 0 {
					invalid = true
					break
				}
				realSellKeys = append(realSellKeys, universalPokers[0].Key)
				universalPokers[0].Key = key
				universalPokers[0].Desc = poker.GetDesc(key)
				universalPokers[0].Val = game.Rules.Value(key)
				sells = append(sells, universalPokers[0])
				universalPokers = universalPokers[1:]
			} else {
				realSellKeys = append(realSellKeys, key)
				sells = append(sells, normalPokers[key][len(normalPokers[key])-1])
				normalPokers[key] = normalPokers[key][:len(normalPokers[key])-1]
			}
		}
		facesArr := poker.ParseFaces(sells, game.Rules)
		if len(facesArr) == 0 {
			invalid = true
		}
		//聊天開啓才能說話
		if invalid {
			if game.Room.EnableChat {
				database.BroadcastChat(player, fmt.Sprintf("%s say: %s\n", player.Name, ans))
				continue
			} else {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
				continue
			}
		}
		lastFaces := game.LastFaces
		if !master && lastFaces != nil {
			if isMax(game, *lastFaces) {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
				continue
			}
			access := false
			for _, faces := range facesArr {
				if isMax(game, faces) || faces.Compare(*lastFaces) {
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
		for _, key := range realSellKeys {
			game.Mnemonic[key]--
		}
		pokers = make(modelx.Pokers, 0)
		for _, curr := range normalPokers {
			pokers = append(pokers, curr...)
		}
		pokers = append(pokers, universalPokers...)
		pokers.SortByOaaValue()
		game.Pokers[player.ID] = pokers
		game.LastPlayer = player.ID
		game.LastFaces = lastFaces
		game.LastPokers = sells
		game.Discards = append(game.Discards, sells...)
		if len(pokers) == 0 {
			database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s, won the game! \n", player.Name, sells.OaaString()))
			room := database.GetRoom(player.RoomID)
			if room != nil {
				room.Lock()
				room.Game = nil
				room.State = consts.RoomStateWaiting
				room.Unlock()
			}
			for _, playerId := range game.Players {
				game.States[playerId] <- stateWaiting
			}
			return nil
		}
		if master {
			playTimes--
			if playTimes > 0 {
				database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s\n", player.Name, sells.OaaString()))
				return playing(player, game, master, playTimes)
			}
		}
		nextPlayer := database.GetPlayer(game.NextPlayer(player.ID))
		database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s, next %s\n", player.Name, sells.OaaString(), nextPlayer.Name))
		game.States[nextPlayer.ID] <- statePlay
		return nil
	}
}

func handlePlay(player *database.Player, game *database.Game) error {
	master := player.ID == game.LastPlayer || game.LastPlayer == 0
	database.Broadcast(player.RoomID, fmt.Sprintf("%s turn to play\n", player.Name))
	if master && game.Room.EnableSkill {
		sk := skill.Skills[consts.SkillID(game.Skills[player.ID])]
		database.Broadcast(player.RoomID, fmt.Sprintf("%s \n", sk.Desc(player)))
		sk.Apply(player, game)
	}
	return playing(player, game, master, game.PlayTimes[player.ID])
}

func InitGame(room *database.Room) (*database.Game, error) {
	rules := rule.LandlordRules
	if !room.EnableLandlord {
		rules = rule.TeamRules
	}

	distributes, decks := poker.Distribute(room.Players, room.EnableDontShuffle, rules)
	players := make([]int64, 0)
	roomPlayers := database.RoomPlayers(room.ID)
	for playerId := range roomPlayers {
		players = append(players, playerId)
	}
	firstOaa := poker.Random(14, 15)
	lastOaa := poker.Random(14, 15, firstOaa)
	states := map[int64]chan int{}
	groups := map[int64]int{}
	pokers := map[int64]modelx.Pokers{}
	skills := map[int64]int{}
	playTimes := map[int64]int{}
	playTimeout := map[int64]time.Duration{}
	mnemonic := map[int]int{
		14: decks,
		15: decks,
	}
	for i := 1; i <= 13; i++ {
		mnemonic[i] = 4 * decks
	}
	for i := range players {
		states[players[i]] = make(chan int, 1)
		groups[players[i]] = 0
		pokers[players[i]] = distributes[i]
		skills[players[i]] = rand.Intn(len(skill.Skills))
		playTimes[players[i]] = 1
		playTimeout[players[i]] = consts.PlayTimeout
	}
	states[players[rand.Intn(len(states))]] <- stateRob
	return &database.Game{
		Room:        room,
		States:      states,
		Players:     players,
		Groups:      groups,
		Pokers:      pokers,
		Additional:  distributes[len(distributes)-1],
		Multiple:    1,
		Universals:  []int{firstOaa, lastOaa},
		Mnemonic:    mnemonic,
		Decks:       decks,
		Skills:      skills,
		PlayTimes:   playTimes,
		PlayTimeOut: playTimeout,
		Rules:       rules,
		Discards:    modelx.Pokers{},
	}, nil
}

func resetGame(game *database.Game) error {
	distributes, decks := poker.Distribute(len(game.Players), game.Room.EnableDontShuffle, game.Rules)
	if len(distributes) != len(game.Players)+1 {
		return consts.ErrorsGamePlayersInvalid
	}
	players := game.Players
	skills := map[int64]int{}
	playTimes := map[int64]int{}
	playTimeout := map[int64]time.Duration{}
	firstOaa := poker.Random(14, 15)
	lastOaa := poker.Random(14, 15, firstOaa)
	for i := range players {
		game.Pokers[players[i]] = distributes[i]
		skills[players[i]] = rand.Intn(len(skill.Skills))
		playTimes[players[i]] = 1
		playTimeout[players[i]] = consts.PlayTimeout
	}
	game.Groups = map[int64]int{}
	game.FirstPlayer = 0
	game.LastPlayer = 0
	game.FirstRob = 0
	game.LastRob = 0
	game.Additional = distributes[len(distributes)-1]
	game.FinalRob = false
	game.Multiple = 1
	game.Universals = []int{firstOaa, lastOaa}
	game.Decks = decks
	game.Skills = skills
	game.PlayTimes = playTimes
	game.PlayTimeOut = playTimeout
	game.Discards = modelx.Pokers{}
	return nil
}

func viewGame(game *database.Game, currPlayer *database.Player) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-20s%-10s%-10s\n", "Name", "Pokers", "Identity"))
	for _, id := range game.Players {
		player := database.GetPlayer(id)
		flag := ""
		if id == currPlayer.ID {
			flag = "*"
		}
		buf.WriteString(fmt.Sprintf("%-20s%-10d%-10s\n", player.Name+flag, len(game.Pokers[id]), game.Team(id)))
	}
	currKeys := map[int]int{}
	for _, currPoker := range game.Pokers[currPlayer.ID] {
		currKeys[currPoker.Key]++
	}
	buf.WriteString("Pokers  : ")
	for _, i := range consts.MnemonicSorted {
		buf.WriteString(poker.GetDesc(i) + "  ")
	}
	buf.WriteString("\nSurplus : ")
	for _, i := range consts.MnemonicSorted {
		buf.WriteString(strconv.Itoa(game.Mnemonic[i]-currKeys[i]) + "  ")
		if i == 10 {
			buf.WriteString(" ")
		}
	}
	if game.Room.EnableLaiZi {
		buf.WriteString("\nThe Universal pokers are: ")
		for _, key := range game.Universals {
			buf.WriteString(poker.GetDesc(key) + " ")
		}
	}
	buf.WriteString("\n")
	_ = currPlayer.WriteString(buf.String())
}

func isMax(game *database.Game, faces modelx.Faces) bool {
	if game.Decks == 1 && len(faces.Keys) == 2 {
		if (faces.Keys[0] == 14 && faces.Keys[1] == 15) || (faces.Keys[0] == 15 && faces.Keys[1] == 14) {
			return true
		}
	}
	return false
}
