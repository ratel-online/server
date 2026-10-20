package game

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ratel-online/core/log"
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/core/util/rand"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/rule"
)

// 够级：6 人 2 副牌，对门（隔 3 个座位）为一队。
// 简化版规则：不含进贡、还贡、革命、开点、憋三；四户乱残后够级牌人人可接；
// 结束条件为一队两人全部出完，或只剩最后一家；按名次结算积分。

const (
	goujiStatePlay    = 7
	goujiStateWaiting = 8
)

// 够级名次与积分（1 名次 = 头科）
var goujiRankNames = []string{"头科", "二科", "三科", "四科", "五落", "大落"}
var goujiRankScores = []int{300, 200, 100, -100, -200, -300}

type GoujiGame struct {
	Room        *database.Room          `json:"room"`
	Players     []int64                 `json:"players"` // 座位顺序
	States      map[int64]chan int      `json:"states"`
	Hands       map[int64]modelx.Pokers `json:"hands"`
	Groups      map[int64]int           `json:"groups"` // 0/1/2 队伍，座位 i 与 i+3 同队
	LastPlayer  int64                   `json:"lastPlayer"`
	LastHand    *rule.GoujiHand         `json:"lastHand"`
	LastCards   modelx.Pokers           `json:"lastCards"`
	DoneRanks   []int64                 `json:"doneRanks"` // 出完牌的名次顺序
	Discards    modelx.Pokers           `json:"discards"`
	PlayTimeOut map[int64]time.Duration `json:"playTimeOut"`
}

func (g *GoujiGame) Clean() {
	if g != nil {
		for _, state := range g.States {
			close(state)
		}
	}
}

// NextPlayer 返回下一个座位（逆时针）。
func (g *GoujiGame) NextPlayer(curr int64) int64 {
	idx := -1
	for i, id := range g.Players {
		if id == curr {
			idx = i
			break
		}
	}
	return g.Players[(idx+1)%len(g.Players)]
}

// Opponent 对门（队友）。
func (g *GoujiGame) Opponent(curr int64) int64 {
	idx := -1
	for i, id := range g.Players {
		if id == curr {
			idx = i
			break
		}
	}
	return g.Players[(idx+3)%len(g.Players)]
}

func (g *GoujiGame) Team(playerId int64) string {
	return "team" + fmt.Sprint(g.Groups[playerId])
}

func (g *GoujiGame) IsDone(playerId int64) bool {
	for _, id := range g.DoneRanks {
		if id == playerId {
			return true
		}
	}
	return false
}

func (g *GoujiGame) activePlayers() []int64 {
	active := make([]int64, 0)
	for _, id := range g.Players {
		if !g.IsDone(id) {
			active = append(active, id)
		}
	}
	return active
}

type Gouji struct{}

func (g *Gouji) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	game := room.Game.(*GoujiGame)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("Game starting! Your team: %s\n", game.Team(player.ID)))
	buf.WriteString(fmt.Sprintf("Your pokers: %s\n", game.Hands[player.ID].String()))
	_ = player.WriteString(buf.String())
	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[Gouji.Next] Player %d (Room %d) loop count: %d, room.State: %d\n", player.ID, player.RoomID, loopCount, room.State)
		}
		if room.State == consts.RoomStateWaiting {
			log.Infof("[Gouji.Next] Player %d exiting, room state changed to waiting, loop count: %d\n", player.ID, loopCount)
			return consts.StateWaiting, nil
		}
		state := <-game.States[player.ID]
		if player.HandBackIfRetired(game.States[player.ID], state) {
			return 0, consts.ErrorsChanClosed
		}
		switch state {
		case goujiStatePlay:
			err := goujiPlaying(player, game)
			if err != nil {
				log.Error(err)
				return 0, err
			}
		case goujiStateWaiting:
			return consts.StateWaiting, nil
		default:
			return 0, consts.ErrorsChanClosed
		}
	}
}

func (*Gouji) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

// goujiNextSeat 返回 from 之后第一个未出完的座位。
func goujiNextSeat(game *GoujiGame, from int64) int64 {
	next := game.NextPlayer(from)
	for i := 0; i < len(game.Players) && game.IsDone(next); i++ {
		next = game.NextPlayer(next)
	}
	return next
}

// goujiRoute 出完牌后的路由：决定谁获得下一个出牌权。
// 普通牌按座位顺序依次要牌，转回出牌者则由其继续；够级牌只有对门可接。
func goujiRoute(player *database.Player, game *GoujiGame) error {
	active := game.activePlayers()
	if len(active) == 0 {
		return nil
	}
	// 够级牌且未乱残：只有对门可以接；对门不在（已出完）则人人可接
	if game.LastHand != nil && game.LastHand.Gouji && len(active) > 4 {
		opp := game.Opponent(game.LastPlayer)
		if !game.IsDone(opp) {
			if rule.CanBeatAny(game.Hands[opp], game.LastHand) {
				game.States[opp] <- goujiStatePlay
				return nil
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("%s can not play, %s continues\n", database.GetPlayer(opp).Name, database.GetPlayer(game.LastPlayer).Name))
			game.States[game.LastPlayer] <- goujiStatePlay
			return nil
		}
	}
	// 普通牌（或乱残）：从下家开始按座位顺序要牌
	next := goujiNextSeat(game, game.LastPlayer)
	if next == game.LastPlayer {
		// 只剩出牌者一人未出完（理论上下面的结算会先触发）
		game.States[next] <- goujiStatePlay
		return nil
	}
	game.States[next] <- goujiStatePlay
	return nil
}

// goujiPass 当前玩家过牌后的路由。
func goujiPass(player *database.Player, game *GoujiGame) error {
	last := game.LastPlayer
	// 够级牌且未乱残：对门不要，其余玩家也不能接，出牌者继续
	if game.LastHand != nil && game.LastHand.Gouji && len(game.activePlayers()) > 4 && !game.IsDone(game.Opponent(last)) {
		game.States[last] <- goujiStatePlay
		return nil
	}
	next := goujiNextSeat(game, player.ID)
	if next == last {
		// 一圈都不要：出牌者获得自由出牌权
		game.States[next] <- goujiStatePlay
		return nil
	}
	game.States[next] <- goujiStatePlay
	return nil
}

func goujiPlaying(player *database.Player, game *GoujiGame) error {
	timer := database.NewTurnTimer(game.PlayTimeOut[player.ID])
	master := game.LastHand == nil || player.ID == game.LastPlayer
	autoAnswer := func() string {
		if master {
			if p := game.Hands[player.ID]; len(p) > 0 {
				return poker.GetAlias(p[0].Key)
			}
		}
		return "p"
	}
	loopCount := 0
	for {
		loopCount++
		if loopCount%100 == 0 {
			log.Infof("[goujiPlaying] Player %d (Room %d) loop count: %d, master: %v, remain: %v\n", player.ID, player.RoomID, loopCount, master, timer.Remaining())
		}
		buf := bytes.Buffer{}
		buf.WriteString("\n")
		if !master && game.LastHand != nil {
			handDesc := rule.GoujiHandString(game.LastCards)
			if game.LastHand.Gouji {
				handDesc += " (gouji)"
			}
			buf.WriteString(fmt.Sprintf("Last: %s played %s\n", database.GetPlayer(game.LastPlayer).Name, handDesc))
		}
		buf.WriteString(fmt.Sprintf("Timeout: %ds, pokers: %s\n", int(timer.Remaining().Seconds()), game.Hands[player.ID].String()))
		_ = player.WriteString(buf.String())

		hand := game.Hands[player.ID]
		// 压不过当前牌直接自动过牌，不打扰玩家
		if !master && game.LastHand != nil && !rule.CanBeatAny(hand, game.LastHand) {
			database.Broadcast(player.RoomID, fmt.Sprintf("%s passed\n", player.Name))
			return goujiPass(player, game)
		}

		ans, ok := timer.Ask(player)
		if !ok {
			ans = autoAnswer()
		}
		rawAns := strings.TrimSpace(ans)
		ans = strings.ToLower(rawAns)
	retry:
		if ans == "" {
			_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsPokersFacesInvalid.Error()))
			continue
		} else if ans == "ls" || ans == "v" {
			goujiViewGame(game, player)
			continue
		} else if ans == "p" || ans == "pass" || ans == "不要" {
			if master {
				_ = player.WriteError(consts.ErrorsHaveToPlay)
				continue
			}
			database.Broadcast(player.RoomID, fmt.Sprintf("%s passed\n", player.Name))
			return goujiPass(player, game)
		}

		// 解析输入
		counts := map[int]int{}
		invalid := false
		cards := make(modelx.Pokers, 0)
		for _, alias := range ans {
			key := poker.GetKey(string(alias))
			if key == 0 {
				invalid = true
				break
			}
			counts[key]++
		}
		if !invalid {
			own := map[int]int{}
			for _, card := range hand {
				own[card.Key]++
			}
			for key, cnt := range counts {
				if own[key] < cnt {
					invalid = true
					break
				}
				for i := 0; i < cnt; i++ {
					cards = append(cards, modelx.Poker{Key: key})
				}
			}
		}
		playHand := rule.ParseGoujiHand(cards)
		if invalid || playHand == nil {
			playHand = nil
		}
		if playHand != nil && !master {
			if game.LastHand == nil || !playHand.CanBeat(game.LastHand) {
				playHand = nil
			}
		}
		if playHand == nil {
			// 非法输入：允许当聊天（限流），连续多次按超时兜底处理
			if timer.Invalid() {
				ans = autoAnswer()
				log.Infof("[goujiPlaying] player %d invalid input reached limit, force play %q\n", player.ID, ans)
				if ans == "p" {
					if master {
						continue
					}
					database.Broadcast(player.RoomID, fmt.Sprintf("%s passed\n", player.Name))
					return goujiPass(player, game)
				}
				goto retry
			}
			if game.Room.EnableChat {
				database.BroadcastChat(player, fmt.Sprintf("%s say: ", player.Name), rawAns)
			} else {
				_ = player.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
			}
			continue
		}

		// 扣牌
		game.Hands[player.ID] = deductCards(hand, cards)
		game.LastPlayer = player.ID
		game.LastHand = playHand
		game.LastCards = cards
		game.Discards = append(game.Discards, cards...)

		desc := rule.GoujiHandString(cards)
		if playHand.Gouji {
			desc += " (gouji)"
		}
		if len(game.Hands[player.ID]) == 0 {
			database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s, finished!\n", player.Name, desc))
			return goujiFinish(player, game)
		}
		database.Broadcast(player.RoomID, fmt.Sprintf("%s played %s\n", player.Name, desc))
		return goujiRoute(player, game)
	}
}

func deductCards(hand modelx.Pokers, played modelx.Pokers) modelx.Pokers {
	remaining := make(modelx.Pokers, 0, len(hand))
	toRemove := map[int]int{}
	for _, card := range played {
		toRemove[card.Key]++
	}
	for _, card := range hand {
		if toRemove[card.Key] > 0 {
			toRemove[card.Key]--
			continue
		}
		remaining = append(remaining, card)
	}
	return remaining
}

// goujiFinish 有玩家出完牌：记录名次，判断是否终局并结算。
func goujiFinish(player *database.Player, game *GoujiGame) error {
	game.DoneRanks = append(game.DoneRanks, player.ID)
	active := game.activePlayers()

	// 终局条件 1：某队两人全部出完；条件 2：只剩最后一家
	teamDone := map[int]int{}
	for _, id := range game.DoneRanks {
		teamDone[game.Groups[id]]++
	}
	over := len(active) <= 1
	for _, cnt := range teamDone {
		if cnt >= 2 {
			over = true
		}
	}
	if over {
		return goujiSettle(game)
	}
	// 未终局：出完牌者退出，下家获得自由出牌权。
	// 注意 LastPlayer 必须始终指向未出完的玩家，否则过牌链会永远轮不到"自由出牌"。
	next := goujiNextSeat(game, player.ID)
	game.LastHand = nil
	game.LastCards = modelx.Pokers{}
	game.LastPlayer = next
	game.States[next] <- goujiStatePlay
	return nil
}

// goujiSettle 终局结算：已出完者按出牌顺序定名次，未出完者按剩牌张数排序补足名次。
func goujiSettle(game *GoujiGame) error {
	room := game.Room
	roomId := room.ID
	rankOf := map[int64]int{}
	for i, id := range game.DoneRanks {
		rankOf[id] = i
	}
	rest := game.activePlayers()
	sort.Slice(rest, func(i, j int) bool {
		if len(game.Hands[rest[i]]) != len(game.Hands[rest[j]]) {
			return len(game.Hands[rest[i]]) < len(game.Hands[rest[j]])
		}
		return rest[i] < rest[j]
	})
	for _, id := range rest {
		rankOf[id] = len(game.DoneRanks) + indexOf(rest, id)
	}

	buf := bytes.Buffer{}
	buf.WriteString("\nGame over! Results:\n")
	for _, id := range game.Players {
		rank := rankOf[id]
		score := goujiRankScores[rank]
		p := database.GetPlayer(id)
		delta := p.AddPoints(score)
		buf.WriteString(fmt.Sprintf("%-20s %-6s %5d -> points: %d\n", p.Name, goujiRankNames[rank], delta, p.Points()))
	}
	database.Broadcast(roomId, buf.String())

	room.Game = nil
	room.State = consts.RoomStateWaiting
	for _, playerId := range game.Players {
		game.States[playerId] <- goujiStateWaiting
	}
	return nil
}

func indexOf(arr []int64, target int64) int {
	for i, v := range arr {
		if v == target {
			return i
		}
	}
	return -1
}

func goujiViewGame(game *GoujiGame, currPlayer *database.Player) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%-20s%-8s%-10s%-8s\n", "Name", "Hand", "Team", "Rank"))
	for _, id := range game.Players {
		player := database.GetPlayer(id)
		flag := ""
		if id == currPlayer.ID {
			flag = "*"
		}
		rank := "-"
		for r, doneId := range game.DoneRanks {
			if doneId == id {
				rank = goujiRankNames[r]
			}
		}
		buf.WriteString(fmt.Sprintf("%-20s%-8d%-10s%-8s\n", player.Name+flag, len(game.Hands[id]), game.Team(id), rank))
	}
	if game.LastHand != nil {
		desc := rule.GoujiHandString(game.LastCards)
		if game.LastHand.Gouji {
			desc += " (gouji)"
		}
		buf.WriteString(fmt.Sprintf("Last: %s played %s\n", database.GetPlayer(game.LastPlayer).Name, desc))
	}
	buf.WriteString("\n")
	_ = currPlayer.WriteString(buf.String())
}

// InitGoujiGame 开局：2 副牌洗匀，每人 18 张，持 3 最多者先出。
func InitGoujiGame(room *database.Room) (*GoujiGame, error) {
	roomPlayers := database.RoomPlayers(room.ID)
	if len(roomPlayers) != 6 {
		return nil, consts.ErrorsGamePlayersInvalid
	}
	players := make([]int64, 0)
	for playerId := range roomPlayers {
		players = append(players, playerId)
	}
	// 座位按玩家 ID 稳定排序，保证对门关系确定性
	sort.Slice(players, func(i, j int) bool { return players[i] < players[j] })

	deck := poker.GetDontShuffleBase()
	deck = append(deck, poker.GetDontShuffleBase()...)
	deck.Shuffle(len(deck), 4)
	for i := range deck {
		deck[i].Val = rule.GoujiValue(deck[i].Key)
	}

	states := map[int64]chan int{}
	hands := map[int64]modelx.Pokers{}
	groups := map[int64]int{}
	playTimeout := map[int64]time.Duration{}
	for i, id := range players {
		states[id] = make(chan int, 1)
		hand := make(modelx.Pokers, 18)
		copy(hand, deck[i*18:(i+1)*18])
		hand.SortByValue()
		hands[id] = hand
		groups[id] = i % 3
		playTimeout[id] = consts.PlayTimeout
	}

	// 持 3 最多者先出，平局随机
	var firstPlayer int64
	maxThree := -1
	candidates := make([]int64, 0)
	for _, id := range players {
		cnt := 0
		for _, card := range hands[id] {
			if card.Key == 3 {
				cnt++
			}
		}
		if cnt > maxThree {
			maxThree = cnt
			candidates = []int64{id}
		} else if cnt == maxThree {
			candidates = append(candidates, id)
		}
	}
	firstPlayer = candidates[rand.Intn(len(candidates))]

	game := &GoujiGame{
		Room:        room,
		Players:     players,
		States:      states,
		Hands:       hands,
		Groups:      groups,
		PlayTimeOut: playTimeout,
		Discards:    modelx.Pokers{},
	}
	game.LastPlayer = firstPlayer
	game.LastHand = nil
	states[firstPlayer] <- goujiStatePlay
	database.Broadcast(room.ID, fmt.Sprintf("%s plays first\n", database.GetPlayer(firstPlayer).Name))
	return game, nil
}
