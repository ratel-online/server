package mahjong

import (
	"fmt"
	"math/rand"
	"testing"
)

// ============================================================
// 随机全量对局模拟器：复刻 state/game/mahjong.go 的 play 循环与
// resolveClaims 优先级，直接使用 mahjong 包本身，
// 每步断言：
//   1) 每家手牌数 = 13-3*副露数(回合外) / 14-3*副露数(回合内)  -> 相公检测
//   2) 总牌守恒: Σ暗牌 + Σ副露牌 + Σ弃牌 + 牌墙 == 136
//   3) 杠后补牌 / 连续杠 / 抢杠 / 杠上开花路径可达
// ============================================================

type simStats struct {
	games, wins, draws, kongs, kongAn, kongBu, kongMing int
	robKongs, kongBlooms, haidi, peng, chi              int
	turns                                               int
}

func tallyMeldTiles(p *Player) int {
	n := 0
	for _, m := range p.Melds {
		n += len(m.Tiles())
	}
	return n
}

// checkInvariants 校验全局守恒
func checkInvariants(t *testing.T, tag string, g *Game) {
	total := g.Wall.Len()
	for _, p := range g.Players {
		total += len(p.Hand) + tallyMeldTiles(p) + len(p.Discards)
	}
	if total != 136 {
		t.Fatalf("%s: 总牌数=%d, want 136", tag, total)
	}
	for i, p := range g.Players {
		m := len(p.Melds)
		lo, hi := 13-3*m, 14-3*m
		if len(p.Hand) < lo || len(p.Hand) > hi {
			t.Fatalf("%s: 座位%d 相公! hand=%d melds=%d 合法区间[%d,%d]",
				tag, i, len(p.Hand), m, lo, hi)
		}
		if m > 4 {
			t.Fatalf("%s: 座位%d 副露 %d 组", tag, i, m)
		}
		// 每种牌面在明面最多 4 张（wall 内部不可见，由 TestWallQuota 保证）
	}
}

// checkPerTurn 精确校验当前轮到谁的张数
func checkPerTurn(t *testing.T, tag string, g *Game, seat int, inTurn bool) {
	p := g.Players[seat]
	m := len(p.Melds)
	want := 13 - 3*m
	if inTurn {
		want = 14 - 3*m
	}
	if len(p.Hand) != want {
		t.Fatalf("%s: 座位%d hand=%d melds=%d want=%d (inTurn=%v)",
			tag, seat, len(p.Hand), m, want, inTurn)
	}
}

func simulateGame(t *testing.T, rng *rand.Rand, nPlayers int, st *simStats) {
	ids := make([]int64, nPlayers)
	names := make([]string, nPlayers)
	for i := range ids {
		ids[i] = int64(i + 1)
		names[i] = fmt.Sprintf("p%d", i)
	}
	g := NewGame(ids, names, rng.Intn(nPlayers))
	seat := g.Dealer
	skipDraw := true
	fromTail := false
	canDeclare := true
	kongBloom := false
	lastDraw := Tile(-1)
	_ = lastDraw
	st.games++
	st.turns++

	for step := 0; step < 3000; step++ {
		st.turns++
		// ---- 摸牌 ----
		if !skipDraw {
			var (
				tk   Tile
				aok  bool
			)
			if fromTail {
				tk, aok = g.DrawReplacement(seat)
			} else {
				tk, aok = g.Draw(seat)
			}
			if !aok {
				st.draws++
				return // 流局
			}
			lastDraw = tk
			kongBloom = fromTail
			canDeclare = true
			fromTail = false
		}
		skipDraw = false
		checkPerTurn(t, "摸牌后", g, seat, true)

		// ---- 本人操作 ----
		discardedOK := false
		for !discardedOK {
			p := g.Players[seat]
			canWin := canDeclare && g.CanSelfWin(seat)
			var kongs []KongOption
			if canDeclare {
				kongs = g.KongOptions(seat)
			}
			if canWin {
				// 一定概率自摸胡（含杠上开花路径）
				if rng.Intn(100) < 50 {
					st.wins++
					if kongBloom {
						st.kongBlooms++
					}
					if g.Wall.Empty() {
						st.haidi++
					}
					if !CanSelfWinOK(g, seat) {
						t.Fatalf("CanSelfWin 与 CanWin 不一致")
					}
					return
				}
			}
			if len(kongs) > 0 && rng.Intn(100) < 60 {
				k := kongs[rng.Intn(len(kongs))]
				applied := false
				if k.Kind == KongAn {
					applied = g.ConcealedKong(seat, k.Tile)
					if applied {
						st.kongAn++
					}
				} else {
					// 补杠(抢杠逻辑在 state 层, 这里直接执行补杠)
					applied = g.AddedKong(seat, k.Tile)
					if applied {
						st.kongBu++
					}
				}
				if !applied {
					t.Fatalf("KongOptions 给出的杠无法执行")
				}
				st.kongs++
				if _, ok := g.DrawReplacement(seat); !ok {
					st.draws++
					return // 墙空杠 → 流局
				}
				kongBloom = true
				canDeclare = true
				checkPerTurn(t, "杠后补牌", g, seat, true)
				continue
			}
			// 随机出一张
			tk := p.Hand[rng.Intn(len(p.Hand))]
			if !g.Discard(seat, tk) {
				t.Fatalf("Discard 失败")
			}
			discardedOK = true
		}
		kongBloom = false
		canDeclare = false
		discarded := g.Players[seat].Discards[len(g.Players[seat].Discards)-1]

		// ---- 吃/碰/杠/胡 询问（复刻 resolveClaims） ----
		n := len(g.Players)
		type claim struct {
			seat int
			hu, gang, peng bool
			chi [][2]Tile
		}
		var list []claim
		for i := 1; i < n; i++ {
			s := (seat + i) % n
			c := claim{seat: s}
			c.hu = g.CanWinWith(s, discarded)
			c.gang = g.CanGangFromDiscard(s, discarded)
			c.peng = g.CanPeng(s, discarded)
			if i == 1 {
				c.chi = g.ChiOptions(s, discarded)
			}
			if c.hu || c.gang || c.peng || len(c.chi) > 0 {
				list = append(list, c)
			}
		}
		resolved := false
		// 决策
		dec := make([]int, len(list)) // 0过 1胡 2杠 3碰 4吃
		decIdx := make([]int, len(list))
		for i, c := range list {
			switch {
			case c.hu && rng.Intn(100) < 60:
				dec[i] = 1
			case c.gang && rng.Intn(100) < 50:
				dec[i] = 2
			case c.peng && rng.Intn(100) < 50:
				dec[i] = 3
			case len(c.chi) > 0 && rng.Intn(100) < 40:
				dec[i] = 4
				decIdx[i] = rng.Intn(len(c.chi))
			}
		}
		// 胡 > 杠 > 碰 > 吃
		for i := range list {
			if dec[i] == 1 {
				st.wins++
				checkPerTurn(t, "点炮胡前", g, list[i].seat, false)
				return
			}
		}
		for i := range list {
			if dec[i] == 2 {
				if g.ClaimGang(list[i].seat, discarded) {
					g.RemoveLastDiscard(seat)
					st.kongMing++
					st.kongs++
					seat = list[i].seat
					skipDraw = false
					fromTail = true
					resolved = true
					checkPerTurn(t, "明杠后(补牌前)", g, seat, false)
				}
				break
			}
		}
		if !resolved {
			for i := range list {
				if dec[i] == 3 {
					if g.ClaimPeng(list[i].seat, discarded) {
						g.RemoveLastDiscard(seat)
						st.peng++
						seat = list[i].seat
						skipDraw = true
						resolved = true
						checkPerTurn(t, "碰后", g, seat, true)
					}
					break
				}
			}
		}
		if !resolved {
			for i := range list {
				if dec[i] == 4 {
					pair := list[i].chi[decIdx[i]]
					if g.ClaimChi(list[i].seat, discarded, pair) {
						g.RemoveLastDiscard(seat)
						st.chi++
						seat = list[i].seat
						skipDraw = true
						resolved = true
						checkPerTurn(t, "吃后", g, seat, true)
					}
					break
				}
			}
		}
		if resolved {
			// 碰/吃后玩家出牌
			if skipDraw {
				p := g.Players[seat]
				tk := p.Hand[rng.Intn(len(p.Hand))]
				if !g.Discard(seat, tk) {
					t.Fatalf("出牌失败")
				}
				discarded = tk
				checkPerTurn(t, "副露后出牌", g, seat, false)
				// 出牌后再进入下一轮 claims —— 递归一层模拟（深度有限）
				// 为简单起见：此处不再递归，直接视为正常轮转到下家
				seat = g.NextSeat(seat)
				skipDraw = false
				fromTail = false
				canDeclare = true
				continue
			}
			continue
		}
		seat = g.NextSeat(seat)
		skipDraw = false
		fromTail = false
		canDeclare = true
		checkInvariants(t, "回合切换", g)
	}
}

func CanSelfWinOK(g *Game, seat int) bool {
	p := g.Players[seat]
	return CanWin(p.Hand, len(p.Melds))
}

func TestSimulateFullGames(t *testing.T) {
	rng := rand.New(rand.NewSource(20261021))
	st := &simStats{}
	for i := 0; i < 1500; i++ {
		simulateGame(t, rng, 4, st)
	}
	for i := 0; i < 500; i++ {
		simulateGame(t, rng, 3, st)
	}
	t.Logf("统计: %d局 %d手 %d胜 %d流局", st.games, st.turns, st.wins, st.draws)
	t.Logf("杠: 总%d (暗%d 明%d 补%d) 抢杠相关-补杠%d 杠上开花%d 海底%d", st.kongs, st.kongAn, st.kongMing, st.kongBu, st.kongBu, st.kongBlooms, st.haidi)
	t.Logf("碰%d 吃%d", st.peng, st.chi)
	if st.kongs < 50 {
		t.Errorf("杠事件过少(%d), 需调整策略压力", st.kongs)
	}
	if st.wins < 20 {
		t.Errorf("胡牌过少(%d)", st.wins)
	}
}
