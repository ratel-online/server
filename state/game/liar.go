package game

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type Liar struct{}

var (
	liarStatePlay    = 1
	liarStateGameEnd = 2
)

func (g *Liar) Next(player *database.Player) (consts.StateID, error) {
	// 这里编写游戏的主要循环逻辑
	// 例如：等待发牌、处理玩家出牌输入、判断胜负等
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.Liar)
	buf := bytes.Buffer{}

	buf.WriteString("欢迎来到骗子酒馆!\n")
	if game.Target != nil {
		buf.WriteString(fmt.Sprintf("当前指示牌: %s\n", poker.GetDesc(game.Target.Key)))
	}
	//获取每位玩家的状态
	buf.WriteString(g.GetPlayerStatus(room))

	_ = game.Bullets[player.ID]
	_ = player.WriteString(buf.String())

	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[Game.Next] Player %d (Room %d) loop count: %d, room.State: %d\n", player.ID, player.RoomID, loopCount, room.State)
		}
		if room.State == consts.RoomStateWaiting {
			log.Infof("[Game.Next] Player %d exiting, room state changed to waiting, loop count: %d\n", player.ID, loopCount)
			return consts.StateWaiting, nil
		}
		log.Infof("[Game.Next] Player %d waiting for state, loop count: %d\n", player.ID, loopCount)
		state := <-game.States[player.ID]
		switch state {
		case liarStatePlay:
			err := g.handlePlay(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case liarStateGameEnd:
			return g.handleGameEnd(player, game)
		}
	}
}

func (g *Liar) handlePlay(player *database.Player, game *database.Liar) error {
	buf := bytes.Buffer{}
	if game.Target != nil {
		buf.WriteString(fmt.Sprintf("\n当前指示牌: %s\n", poker.GetDesc(game.Target.Key)))
	}

	hasLastMove := game.LastPlayerID != 0 && game.LastPlayerID != player.ID
	if hasLastMove {
		lastPlayer := database.GetPlayer(game.LastPlayerID)
		buf.WriteString(fmt.Sprintf("上家 %s 出了 %d 张牌。你可以选择 [质疑(c)] 或 [继续出牌(请输入牌面)]\n", lastPlayer.Name, len(game.LastPokers)))
	}

	buf.WriteString(fmt.Sprintf("你的手牌: %s\n", game.Hands[player.ID].String()))
	_ = player.WriteString(buf.String())

	for {
		ans, err := player.AskForString(consts.PlayTimeout)
		if err != nil || ans == "" {
			// 超时或无输入自动出第一张牌
			ans = poker.GetAlias(game.Hands[player.ID][0].Key)
		}
		ans = strings.TrimSpace(strings.ToLower(ans))

		// 处理质疑逻辑
		if (ans == "c" || ans == "质疑") && hasLastMove {
			g.handleChallenge(player, game)
			return nil
		}

		// 处理出牌逻辑
		keys := make([]int, 0)
		for _, char := range ans {
			key := poker.GetKey(string(char))
			if key != 0 {
				keys = append(keys, key)
			}
		}

		// 检查手牌是否足够
		playedPokers := make(model.Pokers, 0)
		tempHand := make(model.Pokers, len(game.Hands[player.ID]))
		copy(tempHand, game.Hands[player.ID])
		valid := len(keys) > 0

		if valid {
			for _, key := range keys {
				foundIdx := -1
				for i, p := range tempHand {
					if p.Key == key {
						foundIdx = i
						break
					}
				}
				if foundIdx != -1 {
					playedPokers = append(playedPokers, tempHand[foundIdx])
					tempHand = append(tempHand[:foundIdx], tempHand[foundIdx+1:]...)
				} else {
					valid = false
					break
				}
			}
		}

		if !valid {
			// 如果不是有效的质疑或出牌操作，则视为聊天
			database.BroadcastChat(player, fmt.Sprintf("%s 说: %s\n", player.Name, ans))
			continue
		}

		// 更新状态
		game.Hands[player.ID] = tempHand
		game.LastPlayerID = player.ID
		game.LastPokers = playedPokers

		database.Broadcast(player.RoomID, fmt.Sprintf("%s 出了 %d 张牌, 剩余张数: %d\n", player.Name, len(playedPokers), len(game.Hands[player.ID])))

		// 游戏结束判定
		if len(game.Hands[player.ID]) == 0 || g.getAliveCount(game) == 1 {
			for _, id := range game.PlayerIDs {
				game.States[id] <- liarStateGameEnd
			}
			return nil
		}

		// 下一位玩家
		nextID := g.getNextPlayer(game, player.ID)
		game.States[nextID] <- liarStatePlay
		return nil
	}
}

func (g *Liar) handleChallenge(challenger *database.Player, game *database.Liar) {
	lastPlayer := database.GetPlayer(game.LastPlayerID)
	database.Broadcast(game.Room.ID, fmt.Sprintf("%s 质疑了 %s 的出牌！\n", challenger.Name, lastPlayer.Name))
	database.Broadcast(game.Room.ID, fmt.Sprintf("%s 实际上出了: %s\n", lastPlayer.Name, game.LastPokers.String()))

	isLying := false
	for _, p := range game.LastPokers {
		if p.Key != game.Target.Key {
			isLying = true
			break
		}
	}

	var loser *database.Player
	if isLying {
		database.Broadcast(game.Room.ID, fmt.Sprintf("抓到了！%s 确实在撒谎！\n", lastPlayer.Name))
		loser = lastPlayer
	} else {
		database.Broadcast(game.Room.ID, fmt.Sprintf("清白！%s 没有撒谎。%s 质疑失败！\n", lastPlayer.Name, challenger.Name))
		loser = challenger
	}

	// 输家扣动扳机
	isDead := g.pullTrigger(loser, game)

	// 重置出牌状态
	game.LastPlayerID = 0
	game.LastPokers = nil

	if isDead {
		if g.getAliveCount(game) == 1 {
			for _, id := range game.PlayerIDs {
				game.States[id] <- liarStateGameEnd
			}
			return
		}
	}

	// 轮盘赌结束，重新抽取指示牌并对存活玩家重新发牌
	g.resetRound(game)

	// 由输家（如果还活着）或者输家的下一位存活者开始下一轮
	nextID := loser.ID
	if !game.Alive[nextID] {
		nextID = g.getNextPlayer(game, nextID)
	}
	game.States[nextID] <- liarStatePlay
}

func (g *Liar) pullTrigger(player *database.Player, game *database.Liar) bool {
	game.Bong[player.ID]++
	database.Broadcast(game.Room.ID, fmt.Sprintf("%s 满头大汗地拿起了枪，扣动了扳机... (第 %d 次尝试)\n", player.Name, game.Bong[player.ID]))
	if game.Bong[player.ID] == game.Bullets[player.ID] {
		game.Alive[player.ID] = false
		database.Broadcast(game.Room.ID, fmt.Sprintf("砰！！！%s 被子弹贯穿，倒在了地上。\n", player.Name))
		return true
	}
	database.Broadcast(game.Room.ID, fmt.Sprintf("咔哒。是空枪。%s 活了下来，长舒了一口气。\n", player.Name))
	return false
}

func (g *Liar) resetRound(game *database.Liar) {
	deck := initLiarDeck()
	// 重新抽取指示牌
	if len(deck) > 0 {
		game.Target = &deck[0]
		deck = deck[1:]
	}

	// 重新给存活玩家发放手牌
	game.Hands = make(map[int64]model.Pokers)
	aliveIdx := 0
	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			// 确保每个人能分到5张牌（目前20张牌，3个玩家绰绰有余）
			if len(deck) >= (aliveIdx+1)*5 {
				game.Hands[id] = deck[aliveIdx*5 : (aliveIdx+1)*5]
				aliveIdx++
			}
		}
	}
	database.Broadcast(game.Room.ID, "新的一轮开始了！指示牌已更新，存活玩家手牌已重新发放。\n")
}

func (g *Liar) handleGameEnd(player *database.Player, game *database.Liar) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room != nil {
		room.Lock()
		if room.Game != nil {
			winnerID := g.getLastSurvivor(game)
			winnerName := "未知"
			winner := database.GetPlayer(winnerID)
			if winner != nil {
				winnerName = winner.Name
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("游戏结束! %s 获得了胜利!\n", winnerName))
			room.Game = nil
			room.State = consts.RoomStateWaiting
		}
		room.Unlock()
	}

	return consts.StateWaiting, nil
}

func (g *Liar) getAliveCount(game *database.Liar) int {
	count := 0
	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			count++
		}
	}
	return count
}

func (g *Liar) getLastSurvivor(game *database.Liar) int64 {
	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			return id
		}
	}
	return 0
}

func (g *Liar) getNextPlayer(game *database.Liar, curr int64) int64 {
	idx := -1
	for i, id := range game.PlayerIDs {
		if id == curr {
			idx = i
			break
		}
	}
	if idx == -1 {
		return game.PlayerIDs[0]
	}

	// 循环寻找下一个存活的玩家，包括当前玩家（如果只有自己存活）
	for i := 1; i <= len(game.PlayerIDs); i++ {
		nextIdx := (idx + i) % len(game.PlayerIDs)
		nextID := game.PlayerIDs[nextIdx]
		if game.Alive[nextID] {
			return nextID
		}
	}
	return game.PlayerIDs[0]
}

func (g *Liar) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

// 获取房间内所有玩家的状态，显示为玩家名([已开枪次数]/6)
func (g *Liar) GetPlayerStatus(room *database.Room) string {
	buf := bytes.Buffer{}
	game := room.Game.(*database.Liar)
	for _, id := range game.PlayerIDs {
		player := database.GetPlayer(id)
		if player != nil {
			status := "存活"
			if !game.Alive[id] {
				status = "死亡"
			}
			buf.WriteString(fmt.Sprintf("%s (%s) ([%d]/6)\n", player.Name, status, game.Bong[id]))
		}
	}
	return buf.String()
}

func InitLiarGame(room *database.Room) (*database.Liar, error) {
	playerIDs := make([]int64, 0)
	for id := range database.RoomPlayers(room.ID) {
		playerIDs = append(playerIDs, id)
	}
	bullets := make(map[int64]int)
	bong := make(map[int64]int)
	states := make(map[int64]chan int)
	hands := make(map[int64]model.Pokers)
	alive := make(map[int64]bool)
	deck := initLiarDeck()

	// 抽取一张牌作为指示牌
	var target *model.Poker
	if len(deck) > 0 {
		// 随机抽一张作为指示牌，通常不抽大小王作为基础指示牌，但用户没说，我们就直接从牌堆抽第一张
		target = &deck[0]
		deck = deck[1:]
	}

	for i, id := range playerIDs {
		bullets[id] = rand.Intn(6) + 1
		bong[id] = 0
		states[id] = make(chan int, 1)
		alive[id] = true
		// 每个人发5张牌
		hands[id] = deck[i*5 : (i+1)*5]
	}

	// 随机选择一个玩家开始出牌
	states[playerIDs[rand.Intn(len(playerIDs))]] <- liarStatePlay

	return &database.Liar{
		Room:      room,
		PlayerIDs: playerIDs,
		Bullets:   bullets,
		Bong:      bong,
		States:    states,
		Hands:     hands,
		Pokers:    deck,
		Target:    target,
		Alive:     alive,
	}, nil
}

// 初始化牌堆：六张K，六张Q，六张A，一张大王(S)，一张小王(X)
func initLiarDeck() model.Pokers {
	keys := make([]int, 0)
	for i := 0; i < 6; i++ {
		keys = append(keys, 1, 12, 13)
	}
	keys = append(keys, 14, 15)
	pokers := poker.GetPokers(keys...)
	pokers.Shuffle(len(pokers), 1)
	return pokers
}
