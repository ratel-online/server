package game

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/mahjong"
)

// 大众麻将：
//   - 3 人或 4 人开局，136 张牌（万 / 条 / 筒 / 东南西北 / 白发中）
//   - 牌码：W1~W9 万、T1~T9 条、O1~O9 筒、D 东 N 南 X 西 B 北、P 白 F 发 Z 中
//   - 操作：C 吃、P 碰、G 杠、H 胡、0 过
//   - 只有可以吃碰杠胡的玩家才会收到单独提示
const (
	mjClaimTimeout  = 15 * time.Second
	mjActionTimeout = consts.PlayMahjongTimeout
)

// 牌码说明，开局时广播一次
const mjLegend = "牌码：W1-W9 万 | T1-T9 条 | O1-O9 筒 | D 东 N 南 X 西 B 北 | P 白 F 发 Z 中\n" +
	"操作：C 吃 · P 碰 · G 杠 · H 胡 · 0 过\n"

type Mahjong struct{}

func (g *Mahjong) Next(player *database.Player) (consts.StateID, error) {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsExist)
	}
	gd, ok := room.Game.(*database.Mahjong)
	if !ok || gd == nil || !gd.Seated(player.ID) {
		return consts.StateWaiting, nil
	}
	if gd.Finished() {
		return consts.StateWaiting, nil
	}
	// 第一个进入牌桌的玩家负责驱动整局
	if gd.Arrive(player.ID) {
		g.play(room, gd)
		return consts.StateWaiting, nil
	}
	for {
		state, open := <-gd.States[player.ID]
		if !open {
			if gd.HasQuit(player.ID) {
				// 中途退出，直接回到大厅
				return 0, player.WriteError(consts.ErrorsExist)
			}
			return consts.StateWaiting, nil
		}
		// 被顶号换代的旧协程：把这一手交还新会话，自己退出
		if player.HandBackIfRetired(gd.States[player.ID], state) {
			return 0, consts.ErrorsChanClosed
		}
		if state == database.MahjongStateWaiting || gd.Finished() {
			if gd.HasQuit(player.ID) {
				// 中途退出，直接回到大厅
				return 0, player.WriteError(consts.ErrorsExist)
			}
			return consts.StateWaiting, nil
		}
	}
}

func (g *Mahjong) Exit(player *database.Player) consts.StateID {
	room := database.GetRoom(player.RoomID)
	if room == nil {
		return consts.StateHome
	}
	database.Broadcast(room.ID, fmt.Sprintf("%s 离开了牌桌，本局结束。\n", player.Name))
	room.Lock()
	gd, ok := room.Game.(*database.Mahjong)
	if ok {
		room.Game = nil
	}
	room.State = consts.RoomStateWaiting
	room.Unlock()
	if ok && gd != nil {
		gd.Finish()
	}
	database.LeaveRoom(room.ID, player.ID)
	return consts.StateHome
}

type mjActionKind int

const (
	mjActDiscard mjActionKind = iota
	mjActWin
	mjActKongAn
	mjActKongBu
)

type mjAction struct {
	kind mjActionKind
	tile mahjong.Tile
}

// play 驱动整局牌局。只有桌长协程会进入这里。
func (g *Mahjong) play(room *database.Room, gd *database.Mahjong) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[mahjong] room %d panic: %v", room.ID, r)
		}
		g.end(room, gd, -1)
	}()

	if !gd.WaitArrive(10 * time.Second) {
		database.Broadcast(room.ID, "有玩家未能进入牌桌，本局取消。\n")
		return
	}

	gm := gd.Game
	database.Broadcast(room.ID, fmt.Sprintf("=== 大众麻将 开局 === 庄家：%s · 人数：%d 人 · 牌墙：%d 张\n",
		gm.Players[gm.Dealer].Name, len(gm.Players), gm.Wall.Len()))
	database.Broadcast(room.ID, mjLegend)
	for _, p := range gm.Players {
		if dp := database.GetPlayer(p.ID); dp != nil {
			_ = dp.WriteString(fmt.Sprintf("【起手牌】%s\n", mahjong.FormatTiles(p.Hand)))
		}
	}

	seat := gm.Dealer
	skipDraw := true // 庄家起手 14 张
	fromTail := false
	canDeclare := true
	kongBloom := false
	lastDraw := mahjong.Tile(-1)

	for {
		// ---- 摸牌 ----
		if !skipDraw {
			var (
				t  mahjong.Tile
				ok bool
			)
			if fromTail {
				t, ok = gm.DrawReplacement(seat)
			} else {
				t, ok = gm.Draw(seat)
			}
			if !ok {
				g.endWallEmpty(room, gd)
				return
			}
			lastDraw = t
			kongBloom = fromTail
			canDeclare = true
			fromTail = false
			g.notifyDraw(room, gd, seat, t)
		}
		skipDraw = false

		// ---- 本人操作（杠可以连续进行） ----
		var discarded mahjong.Tile
		discardedOK := false
		for !discardedOK {
			act, err := g.askAction(gd, seat, canDeclare, lastDraw)
			if err != nil {
				log.Errorf("[mahjong] room %d player %d action error: %v", room.ID, gm.Players[seat].ID, err)
				if errors.Is(err, consts.ErrorsExist) {
					gd.MarkQuit(gm.Players[seat].ID)
					database.Broadcast(room.ID, fmt.Sprintf("%s 退出牌局，本局结束。\n", gm.Players[seat].Name))
				} else {
					database.Broadcast(room.ID, fmt.Sprintf("%s 掉线，本局结束。\n", gm.Players[seat].Name))
				}
				return
			}
			switch act.kind {
			case mjActWin:
				g.finishWin(room, gd, seat, -1, -1, mahjong.WinContext{
					SelfDraw:  true,
					KongBloom: kongBloom,
					LastTile:  gm.Wall.Empty(),
				})
				return
			case mjActKongAn, mjActKongBu:
				if act.kind == mjActKongBu {
					if robber, ok := g.findRobKong(room, gd, seat, act.tile); ok {
						g.finishWin(room, gd, robber, seat, act.tile, mahjong.WinContext{RobKong: true})
						return
					}
				}
				applied := false
				if act.kind == mjActKongAn {
					applied = gm.ConcealedKong(seat, act.tile)
				} else {
					applied = gm.AddedKong(seat, act.tile)
				}
				if !applied {
					continue
				}
				kindName := "暗杠"
				if act.kind == mjActKongBu {
					kindName = "补杠"
				}
				database.Broadcast(room.ID, fmt.Sprintf("%s %s %s %s\n", gm.Players[seat].Name, kindName, act.tile.Code(), act.tile.Name()))
				t, ok := gm.DrawReplacement(seat)
				if !ok {
					g.endWallEmpty(room, gd)
					return
				}
				lastDraw = t
				kongBloom = true
				canDeclare = true
				g.notifyDraw(room, gd, seat, t)
				continue
			case mjActDiscard:
				if !gm.Discard(seat, act.tile) {
					continue
				}
				discarded = act.tile
				discardedOK = true
			}
		}

		kongBloom = false
		canDeclare = false
		database.Broadcast(room.ID, fmt.Sprintf("%s 打出 %s %s\n", gm.Players[seat].Name, discarded.Code(), discarded.Name()))

		// ---- 吃 / 碰 / 杠 / 胡 ----
		res := g.resolveClaims(room, gd, seat, discarded)
		if res.win {
			return
		}
		if res.claim {
			seat = res.seat
			canDeclare = false
			kongBloom = false
			if res.needDraw {
				skipDraw = false
				fromTail = true
			} else {
				skipDraw = true
			}
			continue
		}
		seat = gm.NextSeat(seat)
		skipDraw = false
		fromTail = false
		canDeclare = true
		kongBloom = false
	}
}

// handWithMark 展示手牌，并给刚摸到的牌标上 *
func handWithMark(p *mahjong.Player, mark mahjong.Tile) string {
	s := mahjong.FormatTiles(p.Hand)
	if !mark.Valid() || !p.HasTile(mark) {
		return s
	}
	return strings.Replace(s, mark.Code(), "*"+mark.Code(), 1)
}

// askAction 询问当前玩家出牌 / 胡 / 杠
func (g *Mahjong) askAction(gd *database.Mahjong, seat int, canDeclare bool, lastDraw mahjong.Tile) (mjAction, error) {
	gm := gd.Game
	p := gm.Players[seat]
	dp := database.GetPlayer(p.ID)
	if dp == nil {
		return mjAction{}, consts.ErrorsExist
	}

	canWin := false
	var kongs []mahjong.KongOption
	mark := mahjong.Tile(-1)
	if canDeclare {
		canWin = gm.CanSelfWin(seat)
		kongs = gm.KongOptions(seat)
		mark = lastDraw // 只有摸牌回合才标记新牌
	}

	buf := &strings.Builder{}
	buf.WriteString(fmt.Sprintf("\n【轮到你】牌墙剩余 %d 张\n", gm.Wall.Len()))
	buf.WriteString(fmt.Sprintf("手牌(%d)：%s\n", len(p.Hand), handWithMark(p, mark)))
	buf.WriteString(fmt.Sprintf("积分：%d\n", dp.Points()))
	if len(p.Melds) > 0 {
		buf.WriteString(fmt.Sprintf("副露：%s\n", mahjong.FormatMelds(p.Melds)))
	}
	if canWin {
		buf.WriteString("可胡：输入 H 自摸胡牌\n")
	}
	if len(kongs) > 0 {
		buf.WriteString("可杠：")
		for i, k := range kongs {
			buf.WriteString(fmt.Sprintf("G%d=%s %s  ", i+1, k.Kind, k.Tile.Code()))
		}
		buf.WriteString("\n")
	}
	buf.WriteString("请出牌（输入牌码，如 W3），v 看牌桌，e 退出；其他内容将作为聊天发送：\n")
	prompt := buf.String()

	// 先进入读取状态再提示，避免玩家秒回时指令被丢弃
	dp.StartTransaction()
	defer dp.StopTransaction()
	deadline := time.Now().Add(mjActionTimeout)
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return g.autoDiscard(p, lastDraw, dp)
		}
		_ = dp.WriteString(prompt)
		ans, err := dp.AskForStringWithoutTransaction(remain)
		if err != nil {
			if errors.Is(err, consts.ErrorsTimeout) {
				return g.autoDiscard(p, lastDraw, dp)
			}
			return mjAction{}, err
		}
		text := strings.TrimSpace(ans)
		if text == "" {
			continue
		}
		cmd := strings.ToUpper(text)
		if cmd == "H" || cmd == "HU" || cmd == "胡" {
			if !canWin {
				_ = dp.WriteString("当前手牌不能胡。\n")
				continue
			}
			return mjAction{kind: mjActWin}, nil
		}
		if cmd == "V" || cmd == "LS" || cmd == "看" {
			_ = dp.WriteString(g.tableView(gd, seat))
			continue
		}
		if strings.HasPrefix(cmd, "G") {
			idx, e := strconv.Atoi(strings.TrimSpace(cmd[1:]))
			if e == nil && idx >= 1 && idx <= len(kongs) {
				k := kongs[idx-1]
				kind := mjActKongAn
				if k.Kind == mahjong.KongBu {
					kind = mjActKongBu
				}
				return mjAction{kind: kind, tile: k.Tile}, nil
			}
			_ = dp.WriteString("没有该杠选项。\n")
			continue
		}
		if t, ok := mahjong.ParseTile(cmd); ok {
			if p.HasTile(t) {
				return mjAction{kind: mjActDiscard, tile: t}, nil
			}
			_ = dp.WriteString(fmt.Sprintf("你手上没有 %s %s。\n", t.Code(), t.Name()))
			continue
		}
		// 既不是牌码也不是指令：当作聊天内容发出去，牌还是得继续打
		if isPassAnswer(cmd) {
			continue
		}
		g.sayChat(gd, seat, text)
	}
}

// autoDiscard 超时自动打出刚摸的牌（没有就打出最后一张）
func (g *Mahjong) autoDiscard(p *mahjong.Player, lastDraw mahjong.Tile, dp *database.Player) (mjAction, error) {
	t := lastDraw
	if !t.Valid() || !p.HasTile(t) {
		if len(p.Hand) == 0 {
			return mjAction{}, consts.ErrorsTimeout
		}
		t = p.Hand[len(p.Hand)-1]
	}
	_ = dp.WriteString(fmt.Sprintf("操作超时，自动打出 %s %s\n", t.Code(), t.Name()))
	return mjAction{kind: mjActDiscard, tile: t}, nil
}

// isPassAnswer 是否是「过」这类没有实际操作的答复
func isPassAnswer(ans string) bool {
	switch ans {
	case "0", "过", "PASS", "NO":
		return true
	}
	return false
}

// sayChat 把未识别的输入当作聊天广播；房间关闭聊天时给出提示
func (g *Mahjong) sayChat(gd *database.Mahjong, seat int, text string) {
	room := gd.Room
	if room == nil {
		return
	}
	p := database.GetPlayer(gd.Game.Players[seat].ID)
	if p == nil {
		return
	}
	if !room.EnableChat {
		_ = p.WriteString(fmt.Sprintf("%s\n", consts.ErrorsChatUnopened.Error()))
		return
	}
	database.BroadcastChat(p, fmt.Sprintf("%s 说: ", p.Name), text)
}

type mjClaim struct {
	seat int
	hu   bool
	gang bool
	peng bool
	chi  [][2]mahjong.Tile
}

type mjClaimResult struct {
	win      bool
	seat     int
	claim    bool
	needDraw bool
}

// resolveClaims 询问所有可以对这张弃牌操作的玩家，并按 胡 > 杠 > 碰 > 吃 的顺序结算
func (g *Mahjong) resolveClaims(room *database.Room, gd *database.Mahjong, from int, tile mahjong.Tile) mjClaimResult {
	gm := gd.Game
	n := len(gm.Players)
	list := make([]mjClaim, 0, n)
	for i := 1; i < n; i++ {
		seat := (from + i) % n
		c := mjClaim{seat: seat}
		c.hu = gm.CanWinWith(seat, tile)
		c.gang = gm.CanGangFromDiscard(seat, tile)
		c.peng = gm.CanPeng(seat, tile)
		if i == 1 {
			// 只有下家可以吃
			c.chi = gm.ChiOptions(seat, tile)
		}
		if c.hu || c.gang || c.peng || len(c.chi) > 0 {
			list = append(list, c)
		}
	}
	if len(list) == 0 {
		return mjClaimResult{}
	}

	seats := make([]int, len(list))
	prompts := make([]string, len(list))
	for i, c := range list {
		seats[i] = c.seat
		prompts[i] = g.claimPrompt(gd, from, tile, c)
	}
	answers := g.askMany(gd, seats, prompts, mjClaimTimeout, func(cmd string) bool {
		switch cmd {
		case "H", "HU", "胡", "G", "GANG", "杠", "P", "PENG", "碰":
			return true
		}
		if strings.HasPrefix(cmd, "C") {
			if _, e := strconv.Atoi(strings.TrimSpace(cmd[1:])); e == nil {
				return true
			}
		}
		return false
	})

	type choice struct {
		act int // 0 过, 1 胡, 2 杠, 3 碰, 4+ 吃
		idx int
	}
	choices := make([]choice, len(list))
	for i, ans := range answers {
		cmd := strings.ToUpper(ans)
		switch {
		case cmd == "H" || cmd == "HU" || cmd == "胡":
			if list[i].hu {
				choices[i] = choice{act: 1}
			}
		case cmd == "G" || cmd == "GANG" || cmd == "杠":
			if list[i].gang {
				choices[i] = choice{act: 2}
			}
		case cmd == "P" || cmd == "PENG" || cmd == "碰":
			if list[i].peng {
				choices[i] = choice{act: 3}
			}
		case strings.HasPrefix(cmd, "C"):
			idx, e := strconv.Atoi(strings.TrimSpace(cmd[1:]))
			if e == nil && idx >= 1 && idx <= len(list[i].chi) {
				choices[i] = choice{act: 4, idx: idx - 1}
			}
		}
	}

	// 胡 > 杠 > 碰 > 吃；同级按逆时针顺序（离出牌者近的优先）
	for i := range list {
		if choices[i].act == 1 {
			g.finishWin(room, gd, list[i].seat, from, tile, mahjong.WinContext{LastTile: gm.Wall.Empty()})
			return mjClaimResult{win: true, seat: list[i].seat}
		}
	}
	for i := range list {
		if choices[i].act == 2 {
			if gm.ClaimGang(list[i].seat, tile) {
				gm.RemoveLastDiscard(from)
				database.Broadcast(room.ID, fmt.Sprintf("%s 明杠 %s %s\n", gm.Players[list[i].seat].Name, tile.Code(), tile.Name()))
				return mjClaimResult{claim: true, seat: list[i].seat, needDraw: true}
			}
		}
	}
	for i := range list {
		if choices[i].act == 3 {
			if gm.ClaimPeng(list[i].seat, tile) {
				gm.RemoveLastDiscard(from)
				database.Broadcast(room.ID, fmt.Sprintf("%s 碰 %s %s\n", gm.Players[list[i].seat].Name, tile.Code(), tile.Name()))
				return mjClaimResult{claim: true, seat: list[i].seat}
			}
		}
	}
	for i := range list {
		if choices[i].act == 4 {
			pair := list[i].chi[choices[i].idx]
			if gm.ClaimChi(list[i].seat, tile, pair) {
				gm.RemoveLastDiscard(from)
				database.Broadcast(room.ID, fmt.Sprintf("%s 吃 %s + %s %s\n", gm.Players[list[i].seat].Name,
					mahjong.FormatTiles([]mahjong.Tile{pair[0], pair[1]}), tile.Code(), tile.Name()))
				return mjClaimResult{claim: true, seat: list[i].seat}
			}
		}
	}
	return mjClaimResult{}
}

// findRobKong 补杠时检查是否有人抢杠胡
func (g *Mahjong) findRobKong(room *database.Room, gd *database.Mahjong, from int, tile mahjong.Tile) (int, bool) {
	gm := gd.Game
	n := len(gm.Players)
	seats := make([]int, 0, n)
	prompts := make([]string, 0, n)
	for i := 1; i < n; i++ {
		seat := (from + i) % n
		if gm.CanWinWith(seat, tile) {
			seats = append(seats, seat)
			prompts = append(prompts, fmt.Sprintf("\n【抢杠】%s 补杠 %s %s\n你可以：\n  H. 抢杠胡\n  0. 过\n请输入指令（其他内容作为聊天发送）：\n",
				gm.Players[from].Name, tile.Code(), tile.Name()))
		}
	}
	if len(seats) == 0 {
		return 0, false
	}
	answers := g.askMany(gd, seats, prompts, mjClaimTimeout, func(cmd string) bool {
		return cmd == "H" || cmd == "HU" || cmd == "胡"
	})
	for i, ans := range answers {
		cmd := strings.ToUpper(ans)
		if cmd == "H" || cmd == "HU" || cmd == "胡" {
			return seats[i], true
		}
	}
	return 0, false
}

// askMany 同时向多名玩家发提示并收集答复（只发给可操作的玩家）。
// valid 判断输入是不是这一步的合法指令；不是指令的内容当作聊天广播，
// 并且不消耗这次决策的时间，玩家聊完还能继续吃碰杠胡。
func (g *Mahjong) askMany(gd *database.Mahjong, seats []int, prompts []string, timeout time.Duration, valid func(string) bool) []string {
	gm := gd.Game
	answers := make([]string, len(seats))
	type resp struct {
		idx int
		ans string
	}
	ch := make(chan resp, len(seats))
	targets := make([]*database.Player, len(seats))
	for i, seat := range seats {
		dp := database.GetPlayer(gm.Players[seat].ID)
		targets[i] = dp
		if dp == nil {
			continue
		}
		// 先进入读取状态，再发提示，避免玩家秒回时输入被丢弃
		dp.StartTransaction()
		if i < len(prompts) {
			_ = dp.WriteString(prompts[i])
		}
	}
	for i := range seats {
		go func(i int, seat int, dp *database.Player) {
			ans := ""
			if dp != nil {
				deadline := time.Now().Add(timeout)
				for {
					remain := time.Until(deadline)
					if remain <= 0 {
						break
					}
					v, err := dp.AskForStringWithoutTransaction(remain)
					if err != nil {
						break
					}
					v = strings.TrimSpace(v)
					if v == "" {
						continue
					}
					cmd := strings.ToUpper(v)
					if isPassAnswer(cmd) || valid(cmd) {
						ans = v
						break
					}
					// 不是指令：当聊天发出去，继续等他的操作
					g.sayChat(gd, seat, v)
					if i < len(prompts) {
						_ = dp.WriteString(prompts[i])
					}
				}
				dp.StopTransaction()
			}
			ch <- resp{idx: i, ans: ans}
		}(i, seats[i], targets[i])
	}
	deadline := time.After(timeout + 5*time.Second)
	for received := 0; received < len(seats); {
		select {
		case r := <-ch:
			answers[r.idx] = strings.TrimSpace(r.ans)
			received++
		case <-deadline:
			return answers
		}
	}
	return answers
}

func (g *Mahjong) claimPrompt(gd *database.Mahjong, from int, tile mahjong.Tile, c mjClaim) string {
	gm := gd.Game
	buf := &strings.Builder{}
	me := gm.Players[c.seat]
	buf.WriteString(fmt.Sprintf("\n【可操作】%s 打出 %s %s\n", gm.Players[from].Name, tile.Code(), tile.Name()))
	buf.WriteString(fmt.Sprintf("你的手牌(%d)：%s\n", len(me.Hand), mahjong.FormatTiles(me.Hand)))
	if dp := database.GetPlayer(me.ID); dp != nil {
		buf.WriteString(fmt.Sprintf("你的积分：%d\n", dp.Points()))
	}
	buf.WriteString("你可以：\n")
	if c.hu {
		buf.WriteString("  H. 胡牌\n")
	}
	if c.gang {
		buf.WriteString("  G. 杠\n")
	}
	if c.peng {
		buf.WriteString("  P. 碰\n")
	}
	for i, pair := range c.chi {
		buf.WriteString(fmt.Sprintf("  C%d. 吃 %s + %s\n", i+1, mahjong.FormatTiles([]mahjong.Tile{pair[0], pair[1]}), tile.Code()))
	}
	buf.WriteString("  0. 过\n请输入指令（超时自动过）；其他内容将作为聊天发送：\n")
	return buf.String()
}

func (g *Mahjong) notifyDraw(room *database.Room, gd *database.Mahjong, seat int, t mahjong.Tile) {
	p := gd.Game.Players[seat]
	if dp := database.GetPlayer(p.ID); dp != nil {
		_ = dp.WriteString(fmt.Sprintf("你摸到 %s %s\n", t.Code(), t.Name()))
	}
	database.Broadcast(room.ID, fmt.Sprintf("%s 摸牌，牌墙剩余 %d 张\n", p.Name, gd.Game.Wall.Len()), p.ID)
}

func (g *Mahjong) tableView(gd *database.Mahjong, viewSeat int) string {
	gm := gd.Game
	buf := &strings.Builder{}
	buf.WriteString(fmt.Sprintf("\n=== 牌桌 === 牌墙剩余 %d 张\n", gm.Wall.Len()))
	for i, p := range gm.Players {
		marks := ""
		if i == gm.Dealer {
			marks += "庄"
		}
		if i == viewSeat {
			marks += "你"
		}
		if marks != "" {
			marks = "(" + marks + ")"
		}
		points := 0
		if dp := database.GetPlayer(p.ID); dp != nil {
			points = dp.Points()
		}
		buf.WriteString(fmt.Sprintf("%d. %s%s 手牌 %d 张 积分 %d 副露：%s\n",
			i+1, p.Name, marks, len(p.Hand), points, mahjong.FormatMelds(p.Melds)))
	}
	for _, p := range gm.Players {
		if len(p.Discards) > 0 {
			buf.WriteString(fmt.Sprintf("%s 弃牌：%s\n", p.Name, mahjong.FormatTiles(p.Discards)))
		}
	}
	buf.WriteString(mjLegend)
	return buf.String()
}

// finishWin 结算胡牌
func (g *Mahjong) finishWin(room *database.Room, gd *database.Mahjong, winner, from int, tile mahjong.Tile, ctx mahjong.WinContext) {
	gm := gd.Game
	p := gm.Players[winner]
	hand := append([]mahjong.Tile(nil), p.Hand...)
	if from >= 0 && tile.Valid() {
		hand = append(hand, tile)
		mahjong.SortTiles(hand)
	}
	score := mahjong.Evaluate(hand, p.Melds, ctx)

	buf := &strings.Builder{}
	if from < 0 {
		buf.WriteString(fmt.Sprintf("\n【胡牌】%s 自摸！\n", p.Name))
	} else {
		buf.WriteString(fmt.Sprintf("\n【胡牌】%s 胡了 %s 打出的 %s %s！\n", p.Name, gm.Players[from].Name, tile.Code(), tile.Name()))
	}
	buf.WriteString(fmt.Sprintf("暗牌：%s\n", mahjong.FormatTiles(hand)))
	if len(p.Melds) > 0 {
		buf.WriteString(fmt.Sprintf("副露：%s\n", mahjong.FormatMelds(p.Melds)))
	}
	buf.WriteString(fmt.Sprintf("番型：%s = %d 番（1 番 %d 积分，共 %d 积分）\n",
		strings.Join(score.Items, " · "), score.Fan, mahjong.PointsPerFan, score.Points))
	unit := score.Points
	if from < 0 {
		buf.WriteString(fmt.Sprintf("结算：自摸，其余 %d 家各付 %d 积分\n", len(gm.Players)-1, unit))
	} else {
		buf.WriteString(fmt.Sprintf("结算：%s 放炮，付 %d 积分\n", gm.Players[from].Name, unit))
	}

	// ---- 与房间积分系统对接：把番数换算成积分并真正结算 ----
	changes := make(map[int64]int, len(gm.Players))
	gained := 0
	short := false
	for i, mp := range gm.Players {
		if i == winner {
			continue
		}
		if from >= 0 && i != from {
			continue
		}
		dp := database.GetPlayer(mp.ID)
		if dp == nil {
			continue
		}
		if dp.Points() < unit {
			short = true
		}
		paid := -dp.AddPoints(-unit)
		changes[mp.ID] = -paid
		gained += paid
	}
	if winnerPlayer := database.GetPlayer(p.ID); winnerPlayer != nil {
		winnerPlayer.AddPoints(gained)
		changes[p.ID] += gained
	}

	buf.WriteString("积分：")
	for i, mp := range gm.Players {
		if i > 0 {
			buf.WriteString(" | ")
		}
		delta := changes[mp.ID]
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		cur := 0
		if dp := database.GetPlayer(mp.ID); dp != nil {
			cur = dp.Points()
		}
		buf.WriteString(fmt.Sprintf("%s %s%d → %d", mp.Name, sign, delta, cur))
	}
	buf.WriteString("\n")
	if short {
		buf.WriteString("（有玩家积分不足，已按剩余积分支付）\n")
	}
	database.Broadcast(room.ID, buf.String())

	g.end(room, gd, winner)
}

func (g *Mahjong) endWallEmpty(room *database.Room, gd *database.Mahjong) {
	database.Broadcast(room.ID, "\n【流局】牌墙已摸完，本局无人胡牌，积分不变。\n")
	g.end(room, gd, gd.Game.Dealer)
}

// end 结束本局并让所有玩家回到房间等待状态
func (g *Mahjong) end(room *database.Room, gd *database.Mahjong, banker int) {
	gd.Finish()
	if !gd.MarkEnded() {
		return
	}
	room.Lock()
	if cur, ok := room.Game.(*database.Mahjong); ok && cur == gd {
		room.Game = nil
	}
	room.State = consts.RoomStateWaiting
	if banker >= 0 && banker < len(gd.Seats) {
		room.Banker = int(gd.Seats[banker])
	}
	room.Unlock()

	database.Broadcast(room.ID, "牌局结束，输入 s 可开始下一局。\n")
}

// InitMahjongGame 创建一局新牌局
func InitMahjongGame(room *database.Room) (*database.Mahjong, error) {
	roomPlayers := database.RoomPlayers(room.ID)
	ids := make([]int64, 0, len(roomPlayers))
	for id := range roomPlayers {
		ids = append(ids, id)
	}
	if len(ids) < 3 || len(ids) > 4 {
		return nil, consts.ErrorsGamePlayersInvalid
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	names := make([]string, len(ids))
	states := make(map[int64]chan int, len(ids))
	for i, id := range ids {
		if p := database.GetPlayer(id); p != nil {
			names[i] = p.Name
		}
		states[id] = make(chan int, 1)
	}

	dealer := 0
	if room.Banker != 0 {
		for i, id := range ids {
			if int(id) == room.Banker {
				dealer = i
				break
			}
		}
	}
	gm := mahjong.NewGame(ids, names, dealer)
	return &database.Mahjong{
		Room:   room,
		Seats:  ids,
		States: states,
		Game:   gm,
	}, nil
}
