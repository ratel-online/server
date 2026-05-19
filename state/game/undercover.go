package game

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type Undercover struct{}

var (
	undercoverStateDescribe = 1 // 描述阶段
	undercoverStateVote     = 2 // 投票阶段
	undercoverStateGameEnd  = 3 // 游戏结束
)

func (g *Undercover) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}

	// 检查房间状态，如果已经回到等待状态则直接返回
	if room.State == consts.RoomStateWaiting {
		return consts.StateWaiting, nil
	}

	game := room.Game.(*database.Undercover)

	// 显示游戏信息
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("\n========== 谁是卧底 - 第%d轮 ==========\n", game.Round))
	buf.WriteString(g.GetPlayerStatus(room, player.ID))
	_ = player.WriteString(buf.String())

	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[Undercover.Next] Player %d (Room %d) loop count: %d, room.State: %d\n", player.ID, player.RoomID, loopCount, room.State)
		}

		if room.State == consts.RoomStateWaiting {
			log.Infof("[Undercover.Next] Player %d exiting, room state changed to waiting, loop count: %d\n", player.ID, loopCount)
			return consts.StateWaiting, nil
		}

		if game.GameOver {
			return g.handleGameEnd(player, game)
		}

		log.Infof("[Undercover.Next] Player %d waiting for state, loop count: %d\n", player.ID, loopCount)
		state := <-game.States[player.ID]
		switch state {
		case undercoverStateDescribe:
			err := g.handleDescribe(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case undercoverStateVote:
			err := g.handleVote(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case undercoverStateGameEnd:
			return g.handleGameEnd(player, game)
		}
	}
}

// handleDescribe 处理描述阶段
func (g *Undercover) handleDescribe(player *database.Player, game *database.Undercover) error {
	// 如果玩家已淘汰，跳过
	if !game.Alive[player.ID] {
		g.nextPlayerOrPhase(game)
		return nil
	}

	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("\n>>> 轮到你了！你的词是：【%s】\n", game.Words[player.ID]))
	buf.WriteString("请输入你对这个词的描述（输入 's' 或 '结束' 结束发言）：\n")
	_ = player.WriteString(buf.String())

	for {
		ans, err := player.AskForString(60 * time.Second)
		if err != nil {
			if err == consts.ErrorsTimeout {
				_ = player.WriteString("发言超时，自动结束发言。\n")
				game.Descriptions[player.ID] = "（超时无描述）"
				database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s: （超时无描述）\n",
					game.PlayerNumbers[player.ID], player.Name))
				g.nextPlayerOrPhase(game)
				return nil
			}
			return err
		}

		ans = strings.TrimSpace(ans)
		if ans == "" {
			continue
		}

		// 检查是否结束发言
		if ans == "s" || ans == "结束" || ans == "结束发言" {
			game.Descriptions[player.ID] = "（结束发言）"
			database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 结束了发言\n",
				game.PlayerNumbers[player.ID], player.Name))
			g.nextPlayerOrPhase(game)
			return nil
		}

		// 广播描述，继续等待下一条输入
		database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s: %s\n",
			game.PlayerNumbers[player.ID], player.Name, ans))
	}
}

// nextPlayerOrPhase 切换到下一个玩家或进入投票阶段
func (g *Undercover) nextPlayerOrPhase(game *database.Undercover) {
	// 检查是否在平票补充描述阶段
	if len(game.TiebreakPlayers) > 0 {
		// 平票补充描述阶段：只检查平票玩家是否都已描述
		allTiebreakDescribed := true
		for _, id := range game.TiebreakPlayers {
			if _, ok := game.Descriptions[id]; !ok {
				allTiebreakDescribed = false
				break
			}
		}

		if allTiebreakDescribed {
			// 平票玩家都已描述完毕，清空平票列表，进入投票阶段
			game.TiebreakPlayers = nil
			database.Broadcast(game.Room.ID, "\n>>> 所有人请同时投票！\n")
			for _, id := range game.PlayerIDs {
				if game.Alive[id] {
					game.States[id] <- undercoverStateVote
				}
			}
			return
		}

		// 找到下一个需要描述的平票玩家
		nextIndex := g.getNextTiebreakIndex(game, game.TurnIndex)
		game.TurnIndex = nextIndex
		nextID := game.PlayerIDs[nextIndex]
		game.States[nextID] <- undercoverStateDescribe
		return
	}

	// 正常轮次：检查是否所有存活玩家都已描述
	allDescribed := true
	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			if _, ok := game.Descriptions[id]; !ok {
				allDescribed = false
				break
			}
		}
	}

	if allDescribed {
		// 进入投票阶段 - 通知所有存活玩家同时投票
		database.Broadcast(game.Room.ID, "\n>>> 所有人请同时投票！\n")
		for _, id := range game.PlayerIDs {
			if game.Alive[id] {
				game.States[id] <- undercoverStateVote
			}
		}
		return
	} else {
		// 通知下一个存活玩家
		game.TurnIndex = g.getNextAliveIndex(game, game.TurnIndex)
		nextID := game.PlayerIDs[game.TurnIndex]
		game.States[nextID] <- undercoverStateDescribe
	}
}

// handleVote 处理投票阶段
func (g *Undercover) handleVote(player *database.Player, game *database.Undercover) error {
	// 如果玩家已淘汰或已投票，跳过
	if !game.Alive[player.ID] {
		return nil
	}
	if _, ok := game.Votes[player.ID]; ok {
		_ = player.WriteString("你已投票，请等待其他玩家...\n")
		return nil
	}

	// 检查玩家是否在线
	if !player.IsOnline() {
		// 玩家离线，标记为已投票（空投票，不参与计票）
		game.Votes[player.ID] = 0
		database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 已离线，跳过投票\n",
			game.PlayerNumbers[player.ID], player.Name))
		g.checkAllVoted(game)
		return nil
	}

	// 获取可投票的目标（存活玩家中排除自己）
	targets := make([]int64, 0)
	for _, id := range game.PlayerIDs {
		if game.Alive[id] && id != player.ID {
			targets = append(targets, id)
		}
	}

	if len(targets) == 0 {
		return nil
	}

	buf := bytes.Buffer{}
	buf.WriteString("\n>>> 投票环节\n")
	buf.WriteString("请选择你要投票的玩家编号：\n")
	for _, id := range targets {
		targetPlayer := database.GetPlayer(id)
		if targetPlayer != nil {
			buf.WriteString(fmt.Sprintf("  [%d] %s\n", game.PlayerNumbers[id], targetPlayer.Name))
		}
	}
	buf.WriteString("\n直接输入数字投票（30秒内未投票将自动跳过）：")
	_ = player.WriteString(buf.String())

	for {
		ans, err := player.AskForString(30 * time.Second)
		if err != nil {
			if err == consts.ErrorsTimeout {
				// 超时跳过投票（不投票）
				game.Votes[player.ID] = 0
				database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 投票超时，跳过投票\n",
					game.PlayerNumbers[player.ID], player.Name))
				g.checkAllVoted(game)
				return nil
			}
			// 其他错误（如连接断开）也跳过投票
			game.Votes[player.ID] = 0
			database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 断开连接，跳过投票\n",
				game.PlayerNumbers[player.ID], player.Name))
			g.checkAllVoted(game)
			return nil
		}

		ans = strings.TrimSpace(ans)
		if ans == "" {
			continue
		}

		// 解析投票目标（只接受纯数字）
		var targetNum int
		_, err = fmt.Sscanf(ans, "%d", &targetNum)
		if err != nil {
			_ = player.WriteString("请输入有效的数字编号！\n")
			continue
		}

		// 查找对应玩家
		var targetID int64 = 0
		for _, id := range targets {
			if game.PlayerNumbers[id] == targetNum {
				targetID = id
				break
			}
		}

		if targetID == 0 {
			_ = player.WriteString("无效的玩家编号，请重新输入！\n")
			continue
		}

		// 记录投票
		game.Votes[player.ID] = targetID
		targetPlayer := database.GetPlayer(targetID)
		database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 投票给了 [%d号] %s\n",
			game.PlayerNumbers[player.ID], player.Name,
			game.PlayerNumbers[targetID], targetPlayer.Name))
		
		// 检查是否所有玩家都已投票
		g.checkAllVoted(game)
		return nil
	}
}

// checkAllVoted 检查是否所有存活玩家都已投票
func (g *Undercover) checkAllVoted(game *database.Undercover) {
	game.Lock()
	defer game.Unlock()

	aliveCount := 0
	voteCount := 0
	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			aliveCount++
			if _, ok := game.Votes[id]; ok {
				voteCount++
			}
		}
	}

	if voteCount >= aliveCount {
		// 所有存活玩家已投票，开始计票
		go g.countVotes(game)
	}
}

// countVotes 计票并处理结果
func (g *Undercover) countVotes(game *database.Undercover) {
	// 统计票数（跳过投票值为0的，表示跳过投票）
	voteCount := make(map[int64]int)
	skipCount := 0
	for _, targetID := range game.Votes {
		if targetID == 0 {
			skipCount++
		} else {
			voteCount[targetID]++
		}
	}

	// 找出最高票数
	maxVotes := 0
	for _, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
		}
	}

	// 找出所有得票最高的玩家
	maxVotedPlayers := make([]int64, 0)
	for id, count := range voteCount {
		if count == maxVotes {
			maxVotedPlayers = append(maxVotedPlayers, id)
		}
	}

	buf := bytes.Buffer{}
	buf.WriteString("\n========== 投票结果 ==========\n")
	for id, count := range voteCount {
		player := database.GetPlayer(id)
		if player != nil {
			buf.WriteString(fmt.Sprintf("[%d号] %s: %d票\n", game.PlayerNumbers[id], player.Name, count))
		}
	}
	if skipCount > 0 {
		buf.WriteString(fmt.Sprintf("跳过投票: %d人\n", skipCount))
	}
	database.Broadcast(game.Room.ID, buf.String())

	if len(maxVotedPlayers) > 1 {
		// 平票，加一轮描述
		database.Broadcast(game.Room.ID, fmt.Sprintf("\n>>> 平票！[%d票] 平票玩家需要加一轮描述\n", maxVotes))

		// 设置平票玩家列表
		game.TiebreakPlayers = maxVotedPlayers
		game.Votes = make(map[int64]int64)

		// 找到第一个平票玩家的索引，从他开始描述
		for i, id := range game.PlayerIDs {
			if contains(maxVotedPlayers, id) {
				game.TurnIndex = i
				game.States[id] <- undercoverStateDescribe
				return
			}
		}
	} else {
		// 淘汰得票最高的玩家
		eliminatedID := maxVotedPlayers[0]
		eliminatedPlayer := database.GetPlayer(eliminatedID)
		game.Alive[eliminatedID] = false

		isUndercover := game.IsUndercover[eliminatedID]
		isBlank := game.IsBlankWord[eliminatedID]

		role := "平民"
		if isUndercover {
			role = "卧底"
		} else if isBlank {
			role = "空白词"
		}

		database.Broadcast(game.Room.ID, fmt.Sprintf("\n>>> [%d号] %s 被淘汰！身份是：%s\n",
			game.PlayerNumbers[eliminatedID], eliminatedPlayer.Name, role))

		// 检查游戏是否结束
		if g.checkGameEnd(game) {
			// 通知所有玩家游戏结束
			for _, id := range game.PlayerIDs {
				game.States[id] <- undercoverStateGameEnd
			}
			return
		}

		// 开始下一轮
		game.Round++
		game.Descriptions = make(map[int64]string)
		game.Votes = make(map[int64]int64)
		game.TiebreakPlayers = nil
		game.TurnIndex = 0

		// 通知第一个存活玩家开始描述
		for _, id := range game.PlayerIDs {
			if game.Alive[id] {
				game.States[id] <- undercoverStateDescribe
				return
			}
		}
	}
}

// checkGameEnd 检查游戏是否结束
func (g *Undercover) checkGameEnd(game *database.Undercover) bool {
	aliveUndercover := 0
	aliveNormal := 0
	aliveBlank := 0

	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			if game.IsUndercover[id] {
				aliveUndercover++
			} else if game.IsBlankWord[id] {
				aliveBlank++
			} else {
				aliveNormal++
			}
		}
	}

	// 所有卧底被淘汰，好人获胜
	if aliveUndercover == 0 && aliveBlank == 0 {
		game.GameOver = true
		database.Broadcast(game.Room.ID, "\n🎉 游戏结束！好人获胜！所有卧底已被淘汰！\n")
		g.broadcastAllWords(game)
		return true
	}

	// 最后剩两人还存在卧底，卧底胜利
	aliveTotal := aliveUndercover + aliveNormal + aliveBlank
	if aliveTotal <= 2 && (aliveUndercover > 0 || aliveBlank > 0) {
		game.GameOver = true
		database.Broadcast(game.Room.ID, "\n🎉 游戏结束！卧底获胜！\n")
		g.broadcastAllWords(game)
		return true
	}

	return false
}

// broadcastAllWords 广播所有人的词和身份
func (g *Undercover) broadcastAllWords(game *database.Undercover) {
	buf := bytes.Buffer{}
	buf.WriteString("\n========== 本局词组 ==========\n")
	buf.WriteString(fmt.Sprintf("平民词：%s\n", game.NormalWord))
	buf.WriteString(fmt.Sprintf("卧底词：%s\n", game.UndercoverWord))
	buf.WriteString("\n========== 玩家身份 ==========\n")
	for _, id := range game.PlayerIDs {
		player := database.GetPlayer(id)
		if player != nil {
			role := "平民"
			word := game.NormalWord
			if game.IsUndercover[id] {
				role = "卧底"
				word = game.UndercoverWord
			} else if game.IsBlankWord[id] {
				role = "空白词"
				word = "（空白）"
			}
			status := "存活"
			if !game.Alive[id] {
				status = "淘汰"
			}
			buf.WriteString(fmt.Sprintf("[%d号] %s: %s - %s (%s)\n",
				game.PlayerNumbers[id], player.Name, role, word, status))
		}
	}
	database.Broadcast(game.Room.ID, buf.String())
}

// handleGameEnd 处理游戏结束
func (g *Undercover) handleGameEnd(player *database.Player, game *database.Undercover) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return consts.StateWaiting, nil
	}

	room.Lock()
	if room.Game != nil {
		// 只执行一次清理
		room.Game = nil
		room.State = consts.RoomStateWaiting
		database.Broadcast(room.ID, "\n游戏已结束，等待房主重新开始...\n")
	}
	room.Unlock()

	return consts.StateWaiting, nil
}

// Exit 退出状态
func (g *Undercover) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

// GetPlayerStatus 获取玩家状态
func (g *Undercover) GetPlayerStatus(room *database.Room, playerID int64) string {
	game := room.Game.(*database.Undercover)
	buf := bytes.Buffer{}

	buf.WriteString("\n玩家列表：\n")
	for _, id := range game.PlayerIDs {
		player := database.GetPlayer(id)
		if player != nil {
			status := "存活"
			if !game.Alive[id] {
				status = "淘汰"
			}
			marker := ""
			if id == playerID {
				marker = " (你)"
			}
			buf.WriteString(fmt.Sprintf("  [%d号] %s - %s%s\n",
				game.PlayerNumbers[id], player.Name, status, marker))
		}
	}

	// 显示自己的词
	if word, ok := game.Words[playerID]; ok && game.Alive[playerID] {
		buf.WriteString(fmt.Sprintf("\n你的词：【%s】\n", word))
	}

	return buf.String()
}

// getNextAliveIndex 获取下一个存活玩家的索引
func (g *Undercover) getNextAliveIndex(game *database.Undercover, currentIdx int) int {
	n := len(game.PlayerIDs)
	if game.IsClockwise {
		for i := 1; i <= n; i++ {
			idx := (currentIdx + i) % n
			if game.Alive[game.PlayerIDs[idx]] {
				return idx
			}
		}
	} else {
		for i := 1; i <= n; i++ {
			idx := (currentIdx - i + n) % n
			if game.Alive[game.PlayerIDs[idx]] {
				return idx
			}
		}
	}
	return currentIdx
}

// getNextTiebreakIndex 获取下一个需要描述的平票玩家索引
func (g *Undercover) getNextTiebreakIndex(game *database.Undercover, currentIdx int) int {
	n := len(game.PlayerIDs)
	for i := 1; i <= n; i++ {
		idx := (currentIdx + i) % n
		playerID := game.PlayerIDs[idx]
		// 检查该玩家是否在平票列表中且还未描述
		if contains(game.TiebreakPlayers, playerID) {
			if _, ok := game.Descriptions[playerID]; !ok {
				return idx
			}
		}
	}
	return currentIdx
}

// InitUndercoverGame 初始化谁是卧底游戏
func InitUndercoverGame(room *database.Room) (*database.Undercover, error) {
	playerIDs := make([]int64, 0)
	for id := range database.RoomPlayers(room.ID) {
		playerIDs = append(playerIDs, id)
	}

	// 随机排序玩家
	rand.Shuffle(len(playerIDs), func(i, j int) {
		playerIDs[i], playerIDs[j] = playerIDs[j], playerIDs[i]
	})

	playerCount := len(playerIDs)

	// 获取卧底数量（从房间属性）
	undercoverCount := 1
	if room.UndercoverNum > 0 && room.UndercoverNum < playerCount {
		undercoverCount = room.UndercoverNum
	}

	// 是否开启空白词模式
	blankWordMode := room.BlankWordMode

	// 分配身份
	isUndercover := make(map[int64]bool)
	isBlankWord := make(map[int64]bool)

	// 随机选择卧底
	undercoverIndices := rand.Perm(playerCount)[:undercoverCount]
	for _, idx := range undercoverIndices {
		isUndercover[playerIDs[idx]] = true
		// 开启空白词模式后，所有卧底都变成空白词
		if blankWordMode {
			isBlankWord[playerIDs[idx]] = true
		}
	}

	// 随机选择词组
	wordPair := database.WordPairs[rand.Intn(len(database.WordPairs))]

	// 随机决定是否互换平民词和卧底词（50%概率）
	normalWord := wordPair.NormalWord
	undercoverWord := wordPair.UndercoverWord
	if rand.Intn(2) == 0 {
		normalWord, undercoverWord = undercoverWord, normalWord
	}

	// 分配词（优先判断空白词，空白词卧底看不到词）
	words := make(map[int64]string)
	for _, id := range playerIDs {
		if isBlankWord[id] {
			// 空白词卧底看不到词
			words[id] = "（空白词，请自由发挥）"
		} else if isUndercover[id] {
			words[id] = undercoverWord
		} else {
			words[id] = normalWord
		}
	}

	// 分配号码牌
	playerNumbers := make(map[int64]int)
	for i, id := range playerIDs {
		playerNumbers[id] = i + 1
	}

	// 创建状态通道
	states := make(map[int64]chan int)
	alive := make(map[int64]bool)
	for _, id := range playerIDs {
		states[id] = make(chan int, 1)
		alive[id] = true
	}

	game := &database.Undercover{
		Room:           room,
		PlayerIDs:      playerIDs,
		States:         states,
		Words:          words,
		IsUndercover:   isUndercover,
		IsBlankWord:    isBlankWord,
		Alive:          alive,
		PlayerNumbers:  playerNumbers,
		Round:          1,
		TurnIndex:      0,
		Descriptions:   make(map[int64]string),
		Votes:          make(map[int64]int64),
		NormalWord:     normalWord,
		UndercoverWord: undercoverWord,
		IsClockwise:    true,
		GameOver:       false,
	}

	// 广播游戏开始信息
	buf := bytes.Buffer{}
	buf.WriteString("\n🎮 谁是卧底 游戏开始！\n")
	buf.WriteString(fmt.Sprintf("本局共有 %d 名玩家", playerCount))
	if blankWordMode {
		buf.WriteString(fmt.Sprintf("，%d 名空白词卧底", undercoverCount))
	} else {
		buf.WriteString(fmt.Sprintf("，%d 名卧底", undercoverCount))
	}
	buf.WriteString("\n")
	buf.WriteString("发言顺序：从1号开始顺序发言\n")
	buf.WriteString("\n玩家列表：\n")
	for _, id := range playerIDs {
		player := database.GetPlayer(id)
		if player != nil {
			buf.WriteString(fmt.Sprintf("  [%d号] %s\n", playerNumbers[id], player.Name))
		}
	}
	database.Broadcast(room.ID, buf.String())

	// 通知第一个玩家开始描述
	firstPlayerID := playerIDs[0]
	game.States[firstPlayerID] <- undercoverStateDescribe

	return game, nil
}

// contains 检查切片是否包含元素
func contains(slice []int64, item int64) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// sortInt64Slice 对int64切片排序（用于确定性的遍历顺序）
func sortInt64Slice(slice []int64) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
}
