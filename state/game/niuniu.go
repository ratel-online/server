package game

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

// 状态常量
const (
	stateBet       = 1 // 下注状态
	stateShowCards = 2 // 亮牌状态
	stateWaitingg  = 4 // 等待状态
)

// 称号常量
const (
	MinAmount            = 2000   // 最低补发筹码
	LowIncomeKingCount   = 10     // 低保王次数
	GamblingGodThreshold = 500000 // 赌神门槛
)

// Niuniu 斗牛游戏结构
type Niuniu struct{}

// Next 游戏主循环
func (g *Niuniu) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*database.NiuniuGame)

	// 检查并补发低保
	checkAndIssueLowIncome(player, game)

	buf := bytes.Buffer{}
	buf.WriteString("====================================\n")
	buf.WriteString("    Welcome to the bullfight    \n")
	buf.WriteString("====================================\n")
	banker := database.GetPlayer(int64(room.Banker))
	buf.WriteString(fmt.Sprintf("Banker: %s", banker.Name))
	if title := getPlayerTitle(banker, game); title != "" {
		buf.WriteString(fmt.Sprintf(" [%s]", title))
	}
	buf.WriteString("\n------------------------------------\n")
	buf.WriteString("Waiting for all players to place their bets...\n")

	_ = player.WriteString(buf.String())

	// 游戏主循环
	for {
		if room.State == int(consts.StateWaiting) {
			return consts.StateWaiting, nil
		}

		state := <-game.States[player.ID]
		switch state {
		case stateBet:
			err := handleBet(room, player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case stateShowCards:
			err := handleShowCards(room, player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case stateWaitingg:
			return consts.StateWaiting, nil
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
}

// Exit 退出游戏
func (g *Niuniu) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return consts.StateHome
	}
	game := room.Game.(*database.NiuniuGame)
	if game == nil {
		return consts.StateHome
	}

	// 通知所有玩家游戏结束
	for _, playerId := range game.Players {
		if state, ok := game.States[playerId]; ok {
			select {
			case state <- stateWaiting:
			default:
			}
		}
	}

	database.Broadcast(player.RoomID, fmt.Sprintf("player %s exit, game over!\n", player.Name))
	database.LeaveRoom(player.RoomID, player.ID)

	game.Clean()
	room.Game = nil
	room.State = consts.RoomStateWaiting

	return consts.StateHome
}

// checkAndIssueLowIncome 检查并补发低保
func checkAndIssueLowIncome(player *database.Player, game *database.NiuniuGame) {
	if int(player.Amount) < 0 {
		player.Amount = MinAmount

		// 增加低保次数
		if game.LowIncomeCount == nil {
			game.LowIncomeCount = make(map[int64]int)
		}
		game.LowIncomeCount[player.ID]++

		buf := bytes.Buffer{}
		buf.WriteString("====================================\n")
		buf.WriteString(fmt.Sprintf("💰 System give you chips: %d D chips\n", MinAmount))
		buf.WriteString(fmt.Sprintf("Collection Count: %d次\n", game.LowIncomeCount[player.ID]))

		// 检查是否获得低保王称号
		if game.LowIncomeCount[player.ID] == LowIncomeKingCount {
			buf.WriteString("🏆 Congratulations on receiving the title: 【低保王】\n")
			buf.WriteString("(Accumulated collection of 10 times)\n")
		} else if game.LowIncomeCount[player.ID] > LowIncomeKingCount {
			buf.WriteString("👑 You are 【低保王】\n")
		}
		buf.WriteString("====================================\n")
		_ = player.WriteString(buf.String())
	}
}

// getPlayerTitle 获取玩家称号
func getPlayerTitle(player *database.Player, game *database.NiuniuGame) string {
	// 赌神称号优先
	if player.Amount >= GamblingGodThreshold {
		return "赌神"
	}

	// 低保王称号
	if game.LowIncomeCount != nil && game.LowIncomeCount[player.ID] >= LowIncomeKingCount {
		return "低保王"
	}

	return ""
}

// calculateMaxLoss 计算闲家最大可能输的分数
func calculateMaxLoss(game *database.NiuniuGame) int {
	// 最大牌型基础分是25(五花/五小)
	maxBaseScore := 25

	// 庄家可能的最大基础分
	bankerID := int64(game.Room.Banker)
	if banker, ok := game.PlayerData[bankerID]; ok && len(banker.Cards) == 5 {
		// 如果已经发牌,用庄家实际牌型
		bankerType := banker.AnalyzeCards()
		maxBaseScore = database.GetCardTypeScore(bankerType)
	}

	return maxBaseScore
}

// handleBet 处理下注
func handleBet(room *database.Room, player *database.Player, game *database.NiuniuGame) error {
	// 如果是庄家,跳过下注
	if player.ID == int64(room.Banker) {
		game.BetReady++
		if game.BetReady >= len(game.Players) {
			// 所有人下注完成,开始发牌
			dealCards(game)
			database.Broadcast(room.ID, "\nAll players have placed their bets and start playing cards...\n")
			time.Sleep(time.Second)
			// 通知所有玩家亮牌
			for _, playerId := range game.Players {
				game.States[playerId] <- stateShowCards
			}
		}
		return nil
	}

	// 计算闲家最大可能输的分数
	maxLoss := calculateMaxLoss(game)
	maxAllowedBet := int(player.Amount) / maxLoss
	if maxAllowedBet < 1 {
		maxAllowedBet = 1
	}

	// 闲家下注
	timeout := consts.PlayTimeout
	database.Broadcast(room.ID, fmt.Sprintf("Waiting %s place a bet...\n", player.Name), player.ID)

	for {
		before := time.Now().Unix()
		buf := bytes.Buffer{}
		buf.WriteString("====================================\n")
		buf.WriteString(fmt.Sprintf("your chips: %d", player.Amount))
		if title := getPlayerTitle(player, game); title != "" {
			buf.WriteString(fmt.Sprintf(" [%s]", title))
		}
		buf.WriteString("\n------------------------------------\n")
		buf.WriteString(fmt.Sprintf("最大可能输: %d分/注 \n", maxLoss))
		buf.WriteString(fmt.Sprintf("建议最大下注: %d分 \n", maxAllowedBet))
		buf.WriteString("------------------------------------\n")
		buf.WriteString("Please enter the betting score:\n")
		buf.WriteString("====================================\n")
		_ = player.WriteString(buf.String())

		ans, err := player.AskForString(timeout)
		if err != nil {
			// 超时默认下注安全金额
			defaultBet := 10
			if defaultBet > maxAllowedBet {
				defaultBet = maxAllowedBet
			}
			ans = strconv.Itoa(defaultBet)
		} else {
			timeout -= time.Second * time.Duration(time.Now().Unix()-before)
		}

		ans = strings.TrimSpace(ans)
		betScore, parseErr := strconv.Atoi(ans)

		if parseErr != nil || betScore < 1 {
			_ = player.WriteString("输入无效,请输入正整数!\n")
			continue
		}

		// 检查是否会导致爆仓
		maxPossibleLoss := betScore * maxLoss
		if uint(maxPossibleLoss) > player.Amount {
			_ = player.WriteString(fmt.Sprintf("❌ 下注过高!可能输: %d分, 超过筹码: %d\n", maxPossibleLoss, player.Amount))
			_ = player.WriteString(fmt.Sprintf("💡 建议最大下注: %d分\n", maxAllowedBet))
			continue
		}

		// 设置下注分数
		game.Bets[player.ID] = betScore
		database.Broadcast(room.ID, fmt.Sprintf("%s place a bet %d \n", player.Name, betScore))

		// 标记该玩家已下注
		game.BetReady++

		// 检查是否所有人都下注完成
		if game.BetReady >= len(game.Players) {
			database.Broadcast(room.ID, "\nAll players have placed their bets and start playing cards...\n")
			time.Sleep(time.Second)

			// 发牌
			dealCards(game)

			// 通知所有玩家亮牌
			for _, playerId := range game.Players {
				game.States[playerId] <- stateShowCards
			}
		}

		return nil
	}
}

// handleShowCards 处理亮牌比大小
func handleShowCards(room *database.Room, player *database.Player, game *database.NiuniuGame) error {
	// 显示自己的手牌
	var currentPlayer *database.NiuniuPlayerData
	for _, p := range game.Players {
		pData := game.PlayerData[p]
		if pData.ID == player.ID {
			currentPlayer = pData
			break
		}
	}

	if currentPlayer == nil {
		return player.WriteError(consts.ErrorsExist)
	}

	buf := bytes.Buffer{}
	buf.WriteString("\n====================================\n")
	buf.WriteString("Your hand: " + currentPlayer.ShowCards() + "\n")
	cardType := currentPlayer.AnalyzeCards()
	buf.WriteString("hand pattern: " + database.GetCardTypeName(cardType) + "\n")
	baseScore := database.GetCardTypeScore(cardType)
	buf.WriteString(fmt.Sprintf("Basic card types: %d\n", baseScore))
	buf.WriteString("====================================\n")
	_ = player.WriteString(buf.String())

	// 标记准备完成
	game.ShowReady++

	// 等待所有玩家都看完牌
	if game.ShowReady < len(game.Players) {
		_ = player.WriteString("Waiting for other players...\n")
		return nil
	}

	// 所有人都准备好了,开始结算
	return settleGame(room, game)
}

// dealCards 发牌
func dealCards(game *database.NiuniuGame) {
	// 创建并洗牌
	deck := createDeck()
	shuffleDeck(deck)

	// 每人发5张牌
	cardIdx := 0
	for i := 0; i < 5; i++ {
		for _, playerId := range game.Players {
			pData := game.PlayerData[playerId]
			pData.Cards = append(pData.Cards, deck[cardIdx])
			cardIdx++
		}
	}
}

// settleGame 结算游戏
func settleGame(room *database.Room, game *database.NiuniuGame) error {
	// 比较所有玩家的牌
	results := compareAllPlayers(game)

	// 广播结果
	buf := bytes.Buffer{}
	buf.WriteString("\n")
	buf.WriteString("====================================\n")
	buf.WriteString("          GAME RESULT          \n")
	buf.WriteString("====================================\n")

	for _, playerId := range game.Players {
		pData := game.PlayerData[playerId]
		cardType := pData.AnalyzeCards()
		baseScore := database.GetCardTypeScore(cardType)

		betInfo := ""
		if playerId != int64(game.Room.Banker) {
			betInfo = fmt.Sprintf(" [下注:%d分]", game.Bets[playerId])
		} else {
			betInfo = " [banker]"
		}

		buf.WriteString(fmt.Sprintf("%s%s: %s\n", pData.Name, betInfo, pData.ShowCards()))
		buf.WriteString(fmt.Sprintf("  牌型: %s (基础分:%d)\n", database.GetCardTypeName(cardType), baseScore))
		buf.WriteString("------------------------------------\n")
	}

	buf.WriteString("\n")
	buf.WriteString("====================================\n")
	buf.WriteString("          Score situation          \n")
	buf.WriteString("====================================\n")

	winnerID := int64(0)
	maxScore := int64(-999999999)

	// 新获得称号的玩家
	newTitles := make(map[int64]string)

	for _, playerId := range game.Players {
		pData := game.PlayerData[playerId]
		score := results[playerId]
		pData.Score += score

		// 更新玩家筹码
		player := database.GetPlayer(playerId)
		if player != nil {
			oldAmount := player.Amount
			player.Amount += uint(score)

			// 检查是否新获得赌神称号
			if oldAmount < GamblingGodThreshold && player.Amount >= GamblingGodThreshold {
				newTitles[playerId] = "赌神"
			}
		}

		scoreStr := ""
		if score > 0 {
			scoreStr = fmt.Sprintf("+%d", score)
			if player.Amount > uint(maxScore) {
				maxScore = int64(player.Amount)
				winnerID = playerId
			}
		} else {
			scoreStr = fmt.Sprintf("%d", score)
		}

		titleStr := ""
		if title := getPlayerTitle(player, game); title != "" {
			titleStr = fmt.Sprintf(" [%s]", title)
		}

		buf.WriteString(fmt.Sprintf("%s%s: %s (Total:%d chips:%d)\n",
			pData.Name, titleStr, scoreStr, pData.Score, player.Amount))
	}

	buf.WriteString("====================================\n")

	// 显示新获得的称号
	if len(newTitles) > 0 {
		buf.WriteString("\n")
		buf.WriteString("🎉 称号获得 🎉\n")
		for playerId, title := range newTitles {
			pData := game.PlayerData[playerId]
			player := database.GetPlayer(playerId)
			buf.WriteString(fmt.Sprintf("🏆 %s 获得称号【%s】! chips: %d\n", pData.Name, title, player.Amount))
		}
		buf.WriteString("\n")
	}

	database.Broadcast(room.ID, buf.String())

	// 更新庄家为本局筹码最多的玩家
	if winnerID != 0 {
		room.Banker = int(winnerID)
		winner := game.PlayerData[winnerID]
		database.Broadcast(room.ID, fmt.Sprintf("\n🎰 %s Having the most chips, become a new banker!\n", winner.Name))
	}

	// // // 游戏结束,清理资源
	// // game.Clean()
	// // room.Game = nil
	// // room.State = consts.RoomStateWaiting

	// for _, playerId := range game.Players {
	// 	if state, ok := game.States[playerId]; ok {
	// 		select {
	// 		case state <- stateWaiting:
	// 		default:
	// 		}
	// 	}
	// }

	// 游戏结束,准备进入下一局（不要立刻 Clean！）
	room.State = consts.RoomStateWaiting
	room.Game = nil

	// 直接通知所有玩家回到等待状态，准备下一局
	for _, playerId := range game.Players {
		if ch, ok := game.States[playerId]; ok {
			select {
			case ch <- stateWaiting:
			default:
			}
		}
	}

	// 延迟清理，防止主循环还在读 channel
	go func() {
		time.Sleep(500 * time.Millisecond)
		game.Clean()
	}()

	return nil
}

// InitNiuniuGame 初始化斗牛游戏
func InitNiuniuGame(room *database.Room) (*database.NiuniuGame, error) {
	players := make([]int64, 0, room.Players)
	playerData := make(map[int64]*database.NiuniuPlayerData)
	states := map[int64]chan int{}
	bets := map[int64]int{}
	lowIncomeCount := make(map[int64]int)

	roomPlayers := database.RoomPlayers(room.ID)

	// 创建玩家
	for playerID := range roomPlayers {
		player := database.GetPlayer(playerID)

		// 检查并补发低保
		if int(player.Amount) < 0 {
			player.Amount = MinAmount
		}

		niuniuPlayer := database.NewNiuniuPlayer(player)
		players = append(players, player.ID)
		playerData[player.ID] = niuniuPlayer
		states[player.ID] = make(chan int, 10)
		bets[player.ID] = 0
	}

	// 设置庄家
	if room.Banker == 0 {
		room.Banker = int(players[rand.Intn(len(players))])
	}

	// 庄家下注分数为0
	bets[int64(room.Banker)] = 0

	game := &database.NiuniuGame{
		Room:           room,
		Players:        players,
		PlayerData:     playerData,
		States:         states,
		Bets:           bets,
		BetReady:       0,
		ShowReady:      0,
		LowIncomeCount: lowIncomeCount,
	}

	// 所有玩家进入下注状态
	for _, playerId := range players {
		states[playerId] <- stateBet
	}

	return game, nil
}

// createDeck 创建一副52张扑克牌
func createDeck() []database.Card {
	deck := make([]database.Card, 0, 52)
	for suit := 0; suit < 4; suit++ {
		for point := 1; point <= 13; point++ {
			deck = append(deck, database.Card{Suit: suit, Point: point})
		}
	}
	return deck
}

// shuffleDeck 洗牌
func shuffleDeck(deck []database.Card) {
	for i := len(deck) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

// compareAllPlayers 比较所有玩家,返回每个玩家的得分
func compareAllPlayers(game *database.NiuniuGame) map[int64]int {
	results := make(map[int64]int)

	bankerID := int64(game.Room.Banker)
	banker := game.PlayerData[bankerID]
	bankerType := banker.AnalyzeCards()
	bankerBaseScore := database.GetCardTypeScore(bankerType)

	// 每个闲家与庄家比较
	for _, playerId := range game.Players {
		if playerId == bankerID {
			continue
		}

		pData := game.PlayerData[playerId]
		playerType := pData.AnalyzeCards()
		playerBaseScore := database.GetCardTypeScore(playerType)

		// 获取闲家下注的分数
		betScore := game.Bets[playerId]
		if betScore == 0 {
			betScore = 10 // 默认10分
		}

		cmp := database.CompareCardType(playerType, bankerType)

		if cmp > 0 {
			// 闲家赢: 闲家得分 = 下注分数 × 闲家牌型基础分
			winScore := betScore * playerBaseScore
			results[playerId] += winScore
			results[bankerID] -= winScore
		} else {
			// 庄家赢: 闲家输分 = 下注分数 × 庄家牌型基础分
			loseScore := betScore * bankerBaseScore
			results[playerId] -= loseScore
			results[bankerID] += loseScore
		}
	}

	return results
}
