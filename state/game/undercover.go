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

type undercoverStateSignal struct {
	playerID int64
	state    int
}

var (
	undercoverStateDescribe = 1 // 描述阶段
	undercoverStateReveal   = 2 // 爆词阶段
	undercoverStateVote     = 3 // 投票阶段
	undercoverStateGameEnd  = 4 // 游戏结束
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

		if g.isGameOver(game) {
			return g.handleGameEnd(player, game)
		}

		log.Infof("[Undercover.Next] Player %d waiting for state, loop count: %d\n", player.ID, loopCount)
		state, ok := <-game.States[player.ID]
		if !ok {
			log.Infof("[Undercover.Next] Player %d state channel closed, returning to waiting\n", player.ID)
			return consts.StateWaiting, nil
		}
		switch state {
		case undercoverStateDescribe:
			err := g.handleDescribe(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case undercoverStateReveal:
			err := g.handleReveal(player, game)
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
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
}

// handleDescribe 处理描述阶段
func (g *Undercover) handleDescribe(player *database.Player, game *database.Undercover) error {
	game.Lock()
	alive := game.Alive[player.ID]
	word := game.Words[player.ID]
	playerNumber := game.PlayerNumbers[player.ID]
	_, alreadyDescribed := game.Descriptions[player.ID]
	game.Unlock()

	// 如果玩家已淘汰，跳过
	if !alive {
		g.nextPlayerOrPhase(game)
		return nil
	}
	if alreadyDescribed {
		log.Infof("[Undercover.handleDescribe] Player %d already described, ignore duplicate describe signal\n", player.ID)
		return nil
	}

	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("\n>>> 轮到你了！你的词是：【%s】\n", word))
	buf.WriteString("请输入你对这个词的描述（输入 's' 或 '结束' 结束发言）：\n")
	_ = player.WriteString(buf.String())

	for {
		ans, err := player.AskForString(60 * time.Second)
		if err != nil {
			if err == consts.ErrorsTimeout {
				_ = player.WriteString("发言超时，自动结束发言。\n")
				g.recordDescription(game, player.ID, "（超时无描述）")
				database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s: （超时无描述）\n",
					playerNumber, player.Name))
				g.nextPlayerOrPhase(game)
				return nil
			}
			g.recordDescription(game, player.ID, "（断线无描述）")
			database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 断开连接，自动结束发言\n",
				playerNumber, player.Name))
			g.nextPlayerOrPhase(game)
			return nil
		}

		ans = strings.TrimSpace(ans)
		if ans == "" {
			continue
		}

		// 检查是否结束发言
		if ans == "s" || ans == "结束" || ans == "结束发言" {
			g.recordDescription(game, player.ID, "（结束发言）")
			database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 结束了发言\n",
				playerNumber, player.Name))
			g.nextPlayerOrPhase(game)
			return nil
		}

		// 广播描述，继续等待下一条输入
		database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s: %s\n",
			playerNumber, player.Name, ans))
	}
}

// handleReveal 处理爆词阶段
func (g *Undercover) handleReveal(player *database.Player, game *database.Undercover) error {
	game.Lock()
	playerNumber := game.PlayerNumbers[player.ID]
	_, alreadyRevealed := game.RevealUsed[player.ID]
	isUndercover := game.IsUndercover[player.ID]
	isAlive := game.Alive[player.ID]
	game.Unlock()

	// 如果玩家已淘汰或不是卧底或已使用过爆词，跳过
	if !isAlive || !isUndercover || alreadyRevealed {
		g.nextRevealPlayerOrVote(game)
		return nil
	}

	buf := bytes.Buffer{}
	buf.WriteString("\n>>> 爆词环节！\n")
	buf.WriteString("你是本轮的卧底！你可以选择：\n")
	buf.WriteString("  1. 输入你猜测的平民词（如果猜对直接获胜）\n")
	buf.WriteString("  2. 输入 's' 跳过爆词，进入投票环节\n")
	buf.WriteString("（每个卧底只能爆词一次）\n")
	buf.WriteString("\n请输入你的选择（60秒内）：")
	_ = player.WriteString(buf.String())

	ans, err := player.AskForString(60 * time.Second)
	if err != nil {
		if err == consts.ErrorsTimeout {
			_ = player.WriteString("爆词超时，自动跳过。\n")
			g.nextRevealPlayerOrVote(game)
			return nil
		}
		_ = player.WriteString("连接断开，自动跳过爆词。\n")
		g.nextRevealPlayerOrVote(game)
		return nil
	}

	ans = strings.TrimSpace(ans)
	if ans == "" {
		_ = player.WriteString("输入不能为空，请重新输入或输入 's' 跳过：")
		return g.handleReveal(player, game)
	}

	// 检查是否跳过爆词
	if ans == "s" || ans == "S" {
		_ = player.WriteString("你选择跳过爆词，进入投票环节。\n")
		game.Lock()
		game.RevealUsed[player.ID] = true
		game.Unlock()
		g.nextRevealPlayerOrVote(game)
		return nil
	}

	// 记录爆词
	game.Lock()
	game.RevealUsed[player.ID] = true
	normalWord := game.NormalWord
	roomID := game.Room.ID
	game.Unlock()

	// 检查是否猜对（忽略空格和大小写）
	normalizedAns := strings.ReplaceAll(strings.ToLower(ans), " ", "")
	normalizedNormalWord := strings.ReplaceAll(strings.ToLower(normalWord), " ", "")

	if normalizedAns == normalizedNormalWord {
		// 猜对了，卧底获胜
		_ = player.WriteString(fmt.Sprintf("🎉 恭喜你猜对了！平民词是：【%s】\n", normalWord))
		database.Broadcast(roomID, fmt.Sprintf("\n>>> [%d号] %s 爆词成功！猜对了平民词：【%s】\n>>> 卧底获胜！\n",
			playerNumber, player.Name, normalWord))

		game.Lock()
		game.GameOver = true
		game.RevealWinner = true
		// 通知所有玩家游戏结束
		for _, id := range game.PlayerIDs {
			g.sendStateSignal(game, undercoverStateSignal{playerID: id, state: undercoverStateGameEnd})
		}
		game.Unlock()

		// 广播所有词
		g.broadcastAllWords(game)
	} else {
		// 猜错了，卧底死亡
		_ = player.WriteString(fmt.Sprintf("❌ 猜错了！平民词不是：【%s】\n", ans))
		_ = player.WriteString("你猜错了，身份暴露，死亡！\n")
		database.Broadcast(roomID, fmt.Sprintf("\n>>> [%d号] %s 爆词失败！猜的词是：【%s】（错误）\n>>> 卧底身份暴露，被淘汰！\n",
			playerNumber, player.Name, ans))

		game.Lock()
		game.Alive[player.ID] = false
		game.RevealWinner = false
		game.Unlock()

		// 检查游戏是否结束
		if g.checkGameEnd(game) {
			// 游戏结束，广播所有词
		} else {
			// 进入下一轮描述
			game.Lock()
			game.Round++
			game.Descriptions = make(map[int64]string)
			game.RevealUndercoverIDs = nil // 清空爆词列表
			game.Votes = make(map[int64]int64)
			game.VoteTargets = nil
			game.TiebreakPlayers = nil
			game.VoteCounting = false

			// 找到第一个存活玩家开始描述
			var firstSignal *undercoverStateSignal
			for i, id := range game.PlayerIDs {
				if game.Alive[id] {
					game.TurnIndex = i
					signal := undercoverStateSignal{playerID: id, state: undercoverStateDescribe}
					firstSignal = &signal
					break
				}
			}
			game.Unlock()

			if firstSignal != nil {
				database.Broadcast(roomID, fmt.Sprintf("\n>>> 卧底已被淘汰，游戏继续！进入第%d轮描述\n", game.Round))
				g.sendStateSignal(game, *firstSignal)
			}
		}
	}

	return nil
}

// nextRevealPlayerOrVote 通知下一个需要爆词的卧底，或进入投票阶段
func (g *Undercover) nextRevealPlayerOrVote(game *database.Undercover) {
	game.Lock()

	// 检查是否所有卧底都已完成爆词
	for _, id := range game.PlayerIDs {
		if game.Alive[id] && game.IsUndercover[id] && game.IsBlankWord[id] {
			if !game.RevealUsed[id] {
				// 找到下一个需要爆词的卧底
				signals := make([]undercoverStateSignal, 0)
				signals = append(signals, undercoverStateSignal{playerID: id, state: undercoverStateReveal})
				game.Unlock()
				g.sendStateSignals(game, signals)
				return
			}
		}
	}

	// 所有卧底都已完成爆词，进入投票阶段
	game.VoteTargets = nil
	game.VoteCounting = false
	signals := g.voteSignalsLocked(game)
	game.Unlock()

	broadcastMsg := "\n>>> 所有卧底爆词结束，进入投票环节！\n"
	database.Broadcast(game.Room.ID, broadcastMsg)
	g.sendStateSignals(game, signals)
}

// nextPlayerOrPhase 切换到下一个玩家或进入投票阶段
func (g *Undercover) nextPlayerOrPhase(game *database.Undercover) {
	broadcastMsg := ""
	signals := make([]undercoverStateSignal, 0)

	game.Lock()
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
			// 平票玩家都已描述完毕，进入平票 PK 投票阶段
			if len(game.VoteTargets) == 0 {
				game.VoteTargets = append([]int64(nil), game.TiebreakPlayers...)
			}
			game.TiebreakPlayers = nil
			game.VoteCounting = false
			broadcastMsg = "\n>>> 平票玩家描述结束，其他玩家请对平票玩家投票！\n"
			signals = g.voteSignalsLocked(game)
			game.Unlock()
			database.Broadcast(game.Room.ID, broadcastMsg)
			g.sendStateSignals(game, signals)
			return
		}

		// 找到下一个需要描述的平票玩家
		nextIndex := g.getNextTiebreakIndex(game, game.TurnIndex)
		game.TurnIndex = nextIndex
		nextID := game.PlayerIDs[nextIndex]
		signals = append(signals, undercoverStateSignal{playerID: nextID, state: undercoverStateDescribe})
		game.Unlock()
		g.sendStateSignals(game, signals)
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
		// 检查是否需要进入爆词阶段（只在空白词模式下开启）
		// 爆词规则：空白词卧底可以在描述结束后尝试猜测平民词
		hasUndercoverToReveal := false
		if game.Room.BlankWordMode {
			for _, id := range game.PlayerIDs {
				if game.Alive[id] && game.IsUndercover[id] && game.IsBlankWord[id] {
					if !game.RevealUsed[id] {
						hasUndercoverToReveal = true
						break
					}
				}
			}
		}

		if hasUndercoverToReveal {
			// 找到第一个需要爆词的卧底
			for _, id := range game.PlayerIDs {
				if game.Alive[id] && game.IsUndercover[id] && game.IsBlankWord[id] {
					if !game.RevealUsed[id] {
						signals = append(signals, undercoverStateSignal{playerID: id, state: undercoverStateReveal})
						broadcastMsg = "\n>>> 所有人描述完毕！卧底可以爆词了！\n"
						game.Unlock()
						database.Broadcast(game.Room.ID, broadcastMsg)
						g.sendStateSignals(game, signals)
						return
					}
				}
			}
		}

		// 进入投票阶段 - 通知所有存活玩家同时投票
		game.VoteTargets = nil
		game.VoteCounting = false
		broadcastMsg = "\n>>> 所有人请同时投票！\n"
		signals = g.voteSignalsLocked(game)
		game.Unlock()
		database.Broadcast(game.Room.ID, broadcastMsg)
		g.sendStateSignals(game, signals)
		return
	} else {
		// 通知下一个存活玩家
		game.TurnIndex = g.getNextAliveIndex(game, game.TurnIndex)
		nextID := game.PlayerIDs[game.TurnIndex]
		signals = append(signals, undercoverStateSignal{playerID: nextID, state: undercoverStateDescribe})
	}
	game.Unlock()
	g.sendStateSignals(game, signals)
}

// handleVote 处理投票阶段
func (g *Undercover) handleVote(player *database.Player, game *database.Undercover) error {
	targets, targetNumbers, voterNumber, canVote, alreadyVoted, tiebreakRestricted := g.voteOptions(game, player.ID)
	if alreadyVoted {
		_ = player.WriteString("你已投票，请等待其他玩家...\n")
		return nil
	}
	if !canVote {
		if tiebreakRestricted {
			_ = player.WriteString("本轮平票 PK 由其他玩家投票，请等待结果...\n")
		}
		return nil
	}

	// 检查玩家是否在线
	if !player.IsOnline() {
		// 玩家离线，标记为已投票（空投票，不参与计票）
		recorded, shouldCount := g.recordVote(game, player.ID, 0)
		if recorded {
			database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 已离线，跳过投票\n",
				voterNumber, player.Name))
		}
		if shouldCount {
			g.countVotes(game)
		}
		return nil
	}

	buf := bytes.Buffer{}
	if tiebreakRestricted {
		buf.WriteString("\n>>> 平票 PK 投票环节\n")
	} else {
		buf.WriteString("\n>>> 投票环节\n")
	}
	buf.WriteString("请选择你要投票的玩家编号：\n")
	for _, id := range targets {
		targetPlayer := database.GetPlayer(id)
		if targetPlayer != nil {
			buf.WriteString(fmt.Sprintf("  [%d] %s\n", targetNumbers[id], targetPlayer.Name))
		}
	}
	buf.WriteString("\n直接输入数字投票（30秒内未投票将自动跳过）：")
	_ = player.WriteString(buf.String())

	for {
		ans, err := player.AskForString(30 * time.Second)
		if err != nil {
			if err == consts.ErrorsTimeout {
				// 超时跳过投票（不投票）
				recorded, shouldCount := g.recordVote(game, player.ID, 0)
				if recorded {
					database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 投票超时，跳过投票\n",
						voterNumber, player.Name))
				}
				if shouldCount {
					g.countVotes(game)
				}
				return nil
			}
			// 其他错误（如连接断开）也跳过投票
			recorded, shouldCount := g.recordVote(game, player.ID, 0)
			if recorded {
				database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 断开连接，跳过投票\n",
					voterNumber, player.Name))
			}
			if shouldCount {
				g.countVotes(game)
			}
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
			if targetNumbers[id] == targetNum {
				targetID = id
				break
			}
		}

		if targetID == 0 {
			_ = player.WriteString("无效的玩家编号，请重新输入！\n")
			continue
		}

		// 记录投票
		recorded, shouldCount := g.recordVote(game, player.ID, targetID)
		if !recorded {
			_ = player.WriteString("本轮投票状态已变化，请等待下一步...\n")
			return nil
		}
		targetPlayer := database.GetPlayer(targetID)
		targetName := fmt.Sprintf("%d号玩家", targetNumbers[targetID])
		if targetPlayer != nil {
			targetName = targetPlayer.Name
		}
		database.Broadcast(game.Room.ID, fmt.Sprintf("[%d号] %s 投票给了 [%d号] %s\n",
			voterNumber, player.Name,
			targetNumbers[targetID], targetName))

		if shouldCount {
			g.countVotes(game)
		}
		return nil
	}
}

// checkAllVoted 检查是否所有存活玩家都已投票
func (g *Undercover) checkAllVoted(game *database.Undercover) {
	if g.tryStartVoteCounting(game) {
		g.countVotes(game)
	}
}

// countVotes 计票并处理结果
func (g *Undercover) countVotes(game *database.Undercover) {
	result := bytes.Buffer{}
	followMsg := ""
	signals := make([]undercoverStateSignal, 0)
	broadcastWords := false
	roomID := game.Room.ID

	game.Lock()
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
	sortInt64Slice(maxVotedPlayers)

	result.WriteString("\n========== 投票结果 ==========\n")
	for id, count := range voteCount {
		player := database.GetPlayer(id)
		if player != nil {
			result.WriteString(fmt.Sprintf("[%d号] %s: %d票\n", game.PlayerNumbers[id], player.Name, count))
		}
	}
	if len(voteCount) == 0 {
		result.WriteString("无有效得票\n")
	}
	if skipCount > 0 {
		result.WriteString(fmt.Sprintf("跳过投票: %d人\n", skipCount))
	}

	switch {
	case len(maxVotedPlayers) == 0:
		game.Round++
		game.Descriptions = make(map[int64]string)
		game.Votes = make(map[int64]int64)
		game.VoteTargets = nil
		game.TiebreakPlayers = nil
		game.VoteCounting = false
		if signal, ok := g.firstAliveDescribeSignalLocked(game); ok {
			signals = append(signals, signal)
		}
		followMsg = "\n>>> 本轮无人获得有效票数，无人被淘汰，进入下一轮发言\n"
	case len(maxVotedPlayers) > 1:
		// 平票，加一轮描述
		followMsg = fmt.Sprintf("\n>>> 平票！[%d票] 平票玩家需要加一轮描述\n", maxVotes)

		// 设置平票玩家列表
		game.TiebreakPlayers = append([]int64(nil), maxVotedPlayers...)
		game.VoteTargets = append([]int64(nil), maxVotedPlayers...)
		game.Votes = make(map[int64]int64)
		game.Descriptions = make(map[int64]string) // 清空描述，准备新一轮描述
		game.VoteCounting = false

		// 找到第一个平票玩家的索引，从他开始描述
		for i, id := range game.PlayerIDs {
			if contains(maxVotedPlayers, id) {
				game.TurnIndex = i
				signals = append(signals, undercoverStateSignal{playerID: id, state: undercoverStateDescribe})
				break
			}
		}
	default:
		// 淘汰得票最高的玩家
		eliminatedID := maxVotedPlayers[0]
		eliminatedPlayer := database.GetPlayer(eliminatedID)
		eliminatedName := fmt.Sprintf("%d号玩家", game.PlayerNumbers[eliminatedID])
		if eliminatedPlayer != nil {
			eliminatedName = eliminatedPlayer.Name
		}
		game.Alive[eliminatedID] = false

		role := g.roleOfPlayerLocked(game, eliminatedID)
		followMsg = fmt.Sprintf("\n>>> [%d号] %s 被淘汰！身份是：%s\n",
			game.PlayerNumbers[eliminatedID], eliminatedName, role)

		// 检查游戏是否结束
		gameOver, gameOverMsg := g.checkGameEndLocked(game)
		game.Votes = make(map[int64]int64)
		game.VoteTargets = nil
		game.TiebreakPlayers = nil
		game.VoteCounting = false
		if gameOver {
			if gameOverMsg != "" {
				followMsg += gameOverMsg
			}
			broadcastWords = true
			// 通知所有玩家游戏结束
			for _, id := range game.PlayerIDs {
				signals = append(signals, undercoverStateSignal{playerID: id, state: undercoverStateGameEnd})
			}
		} else {
			// 开始下一轮
			game.Round++
			game.Descriptions = make(map[int64]string)
			if signal, ok := g.firstAliveDescribeSignalLocked(game); ok {
				signals = append(signals, signal)
			}
		}
	}
	game.Unlock()

	database.Broadcast(roomID, result.String())
	if followMsg != "" {
		database.Broadcast(roomID, followMsg)
	}
	if broadcastWords {
		g.broadcastAllWords(game)
	}
	g.sendStateSignals(game, signals)
}

// checkGameEnd 检查游戏是否结束
func (g *Undercover) checkGameEnd(game *database.Undercover) bool {
	game.Lock()
	gameOver, gameOverMsg := g.checkGameEndLocked(game)
	game.Unlock()
	if gameOver && gameOverMsg != "" {
		database.Broadcast(game.Room.ID, gameOverMsg)
		g.broadcastAllWords(game)
	}
	return gameOver
}

func (g *Undercover) checkGameEndLocked(game *database.Undercover) (bool, string) {
	if game.GameOver {
		return true, ""
	}

	aliveUndercover := 0
	aliveNormal := 0
	aliveBlank := 0

	for _, id := range game.PlayerIDs {
		if game.Alive[id] {
			if game.IsBlankWord[id] {
				aliveBlank++
			} else if game.IsUndercover[id] {
				aliveUndercover++
			} else {
				aliveNormal++
			}
		}
	}

	// 所有卧底被淘汰，好人获胜
	if aliveUndercover == 0 && aliveBlank == 0 {
		game.GameOver = true
		return true, "\n🎉 游戏结束！好人获胜！所有卧底已被淘汰！\n"
	}

	// 最后剩两人还存在卧底，卧底胜利
	aliveTotal := aliveUndercover + aliveNormal + aliveBlank
	if aliveTotal <= 2 && (aliveUndercover > 0 || aliveBlank > 0) {
		game.GameOver = true
		return true, "\n🎉 游戏结束！卧底获胜！\n"
	}

	return false, ""
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
			if game.IsBlankWord[id] {
				role = "空白词"
				word = "（空白）"
			} else if game.IsUndercover[id] {
				role = "卧底"
				word = game.UndercoverWord
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

	// 选择词组，优先使用 chatroom 词库，失败时回退到内置词库
	wordPair, err := database.PickUndercoverWordPair()
	if err != nil {
		log.Errorf("pick undercover word pair fallback: %v", err)
	}

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
		RevealUsed:     make(map[int64]bool),
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
	// 只在空白词模式下显示爆词规则
	if blankWordMode {
		buf.WriteString("🔓 爆词规则：每轮描述结束后，空白词卧底可以猜测平民词\n")
		buf.WriteString("   - 爆词成功（猜对平民词）：卧底获胜\n")
		buf.WriteString("   - 爆词失败（猜错）：卧底死亡，进入下一轮\n")
		buf.WriteString("   - 输入 's' 跳过爆词，进入投票环节\n")
	}
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

func (g *Undercover) isGameOver(game *database.Undercover) bool {
	game.Lock()
	defer game.Unlock()
	return game.GameOver
}

func (g *Undercover) voteOptions(game *database.Undercover, playerID int64) ([]int64, map[int64]int, int, bool, bool, bool) {
	game.Lock()
	defer game.Unlock()

	targetNumbers := make(map[int64]int)
	voterNumber := game.PlayerNumbers[playerID]
	tiebreakRestricted := len(game.VoteTargets) > 0
	if !game.Alive[playerID] {
		return nil, targetNumbers, voterNumber, false, false, tiebreakRestricted
	}
	if _, ok := game.Votes[playerID]; ok {
		return nil, targetNumbers, voterNumber, false, true, tiebreakRestricted
	}
	if !g.isEligibleVoterLocked(game, playerID) {
		return nil, targetNumbers, voterNumber, false, false, tiebreakRestricted
	}

	targets := g.voteTargetsForPlayerLocked(game, playerID)
	for _, id := range targets {
		targetNumbers[id] = game.PlayerNumbers[id]
	}
	return targets, targetNumbers, voterNumber, len(targets) > 0, false, tiebreakRestricted
}

func (g *Undercover) recordVote(game *database.Undercover, voterID, targetID int64) (bool, bool) {
	game.Lock()
	defer game.Unlock()

	if game.GameOver || !game.Alive[voterID] {
		return false, false
	}
	if _, ok := game.Votes[voterID]; ok {
		return false, false
	}
	if !g.isEligibleVoterLocked(game, voterID) {
		return false, false
	}
	if targetID != 0 && !contains(g.voteTargetsForPlayerLocked(game, voterID), targetID) {
		return false, false
	}

	game.Votes[voterID] = targetID
	if g.allVotesInLocked(game) && !game.VoteCounting {
		game.VoteCounting = true
		return true, true
	}
	return true, false
}

func (g *Undercover) tryStartVoteCounting(game *database.Undercover) bool {
	game.Lock()
	defer game.Unlock()

	if game.VoteCounting || !g.allVotesInLocked(game) {
		return false
	}
	game.VoteCounting = true
	return true
}

func (g *Undercover) allVotesInLocked(game *database.Undercover) bool {
	voters := g.voterIDsLocked(game)
	if len(voters) == 0 {
		return false
	}
	for _, id := range voters {
		if _, ok := game.Votes[id]; !ok {
			return false
		}
	}
	return true
}

func (g *Undercover) voteSignalsLocked(game *database.Undercover) []undercoverStateSignal {
	voters := g.voterIDsLocked(game)
	signals := make([]undercoverStateSignal, 0, len(voters))
	for _, id := range voters {
		if len(g.voteTargetsForPlayerLocked(game, id)) > 0 {
			signals = append(signals, undercoverStateSignal{playerID: id, state: undercoverStateVote})
		}
	}
	return signals
}

func (g *Undercover) voterIDsLocked(game *database.Undercover) []int64 {
	voters := make([]int64, 0)
	for _, id := range game.PlayerIDs {
		if g.isEligibleVoterLocked(game, id) {
			voters = append(voters, id)
		}
	}
	return voters
}

func (g *Undercover) isEligibleVoterLocked(game *database.Undercover, playerID int64) bool {
	if !game.Alive[playerID] {
		return false
	}
	if len(game.VoteTargets) == 0 {
		return true
	}
	if g.hasNonTargetVoterLocked(game) {
		return !contains(game.VoteTargets, playerID)
	}
	return true
}

func (g *Undercover) hasNonTargetVoterLocked(game *database.Undercover) bool {
	for _, id := range game.PlayerIDs {
		if game.Alive[id] && !contains(game.VoteTargets, id) {
			return true
		}
	}
	return false
}

func (g *Undercover) voteTargetsForPlayerLocked(game *database.Undercover, playerID int64) []int64 {
	source := game.PlayerIDs
	if len(game.VoteTargets) > 0 {
		source = game.VoteTargets
	}

	targets := make([]int64, 0, len(source))
	for _, id := range source {
		if game.Alive[id] && id != playerID {
			targets = append(targets, id)
		}
	}
	return targets
}

func (g *Undercover) firstAliveDescribeSignalLocked(game *database.Undercover) (undercoverStateSignal, bool) {
	for i, id := range game.PlayerIDs {
		if game.Alive[id] {
			game.TurnIndex = i
			return undercoverStateSignal{playerID: id, state: undercoverStateDescribe}, true
		}
	}
	return undercoverStateSignal{}, false
}

func (g *Undercover) roleOfPlayerLocked(game *database.Undercover, playerID int64) string {
	if game.IsBlankWord[playerID] {
		return "空白词"
	}
	if game.IsUndercover[playerID] {
		return "卧底"
	}
	return "平民"
}

// recordDescription 记录玩家的描述
func (g *Undercover) recordDescription(game *database.Undercover, playerID int64, description string) {
	game.Lock()
	game.Descriptions[playerID] = description
	game.Unlock()
}

// sendStateSignals 向多个玩家发送状态信号
func (g *Undercover) sendStateSignals(game *database.Undercover, signals []undercoverStateSignal) {
	for _, signal := range signals {
		g.sendStateSignal(game, signal)
	}
}

func (g *Undercover) sendStateSignal(game *database.Undercover, signal undercoverStateSignal) {
	defer func() {
		if r := recover(); r != nil {
			log.Infof("[Undercover.sendStateSignal] Skip state %d for player %d because channel is closed: %v\n", signal.state, signal.playerID, r)
		}
	}()

	if ch, ok := game.States[signal.playerID]; ok {
		select {
		case ch <- signal.state:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- signal.state:
			default:
				log.Infof("[Undercover.sendStateSignal] Drop state %d for player %d because channel is still full\n", signal.state, signal.playerID)
			}
		}
	}
}
