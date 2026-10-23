package mahjong

import (
	"fmt"
	"math/rand"
	"testing"
)

// ============================================================
// 随机全量对局模拟器：复刻 state/game/mahjong.go 的 play 循环与
// resolveClaims 优先级，直接使用 mahjong 包本身。
// 与真实服务端一致的流程：
//   - 摸牌(头)/杠后补牌(尾) -> 自摸胡(可跳过)/暗杠/补杠(可跳过)/出牌
//   - 出牌后声明结算: 胡>杠>碰>吃; 碰/吃后打出的牌再次进入声明结算
//     (链式, 与服务端 resolveClaims 的递归行为一致)
// 每步断言（确定性不变量，不依赖随机覆盖率）：
//   1) 每家手牌数 = 13-3*副露数(回合外) / 14-3*副露数(回合内) -> 相公检测
//   2) 总牌守恒: Σ暗牌 + Σ副露牌 + Σ弃牌 + 牌墙 == 136
// 随机源: 通过 NewGameWithRand 注入, 洗牌与决策共用同一 rng,
// 整个模拟完全确定（同版本 Go 下结果可复现）。
// ============================================================

type simStats struct {
	games, wins, draws, kongs, kongAn, kongBu, kongMing int
	kongBlooms, haidi, peng, chi                        int
	turns, maxChain, chains                              int
}

func tallyMeldTiles(p *Player) int {
	n := 0
	for _, m := range p.Melds {
		n += len(m.Tiles())
	}
	return n
}

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
	}
}

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

type simClaim struct {
	seat       int
	hu, gang, peng bool
	chi        [][2]Tile
}

// buildClaims 复刻 resolveClaims 的候选构造(只有下家可吃)
func buildClaims(g *Game, from int, tile Tile) []simClaim {
	n := len(g.Players)
	list := make([]simClaim, 0, n)
	for i := 1; i < n; i++ {
		s := (from + i) % n
		c := simClaim{seat: s}
		c.hu = g.CanWinWith(s, tile)
		c.gang = g.CanGangFromDiscard(s, tile)
		c.peng = g.CanPeng(s, tile)
		if i == 1 {
			c.chi = g.ChiOptions(s, tile)
		}
		if c.hu || c.gang || c.peng || len(c.chi) > 0 {
			list = append(list, c)
		}
	}
	return list
}

// settleClaims 按服务端 resolveClaims 的 胡>杠>碰>吃 顺序结算。
// dec[i]: 0过 1胡 2杠 3碰 4吃(配 decIdx 选吃法)。
// 返回 (结算是否发生, 操作座位, 是否需要杠后补牌)。
// 注意: 胡牌分支只负责座位判定与状态变更语义, 积分/广播在 state 层。
func settleClaims(t *testing.T, g *Game, from int, tile Tile, list []simClaim, dec []int, decIdx []int) (bool, int, bool) {
	// 胡
	for i := range list {
		if dec[i] == 1 {
			return true, list[i].seat, false
		}
	}
	// 杠(失败则继续尝试下一个可杠者)
	for i := range list {
		if dec[i] == 2 {
			if g.ClaimGang(list[i].seat, tile) {
				g.RemoveLastDiscard(from)
				return true, list[i].seat, true
			}
		}
	}
	// 碰
	for i := range list {
		if dec[i] == 3 {
			if g.ClaimPeng(list[i].seat, tile) {
				g.RemoveLastDiscard(from)
				return true, list[i].seat, false
			}
		}
	}
	// 吃
	for i := range list {
		if dec[i] == 4 {
			if decIdx[i] < 0 || decIdx[i] >= len(list[i].chi) {
				t.Fatalf("吃法下标越界")
			}
			if g.ClaimChi(list[i].seat, tile, list[i].chi[decIdx[i]]) {
				g.RemoveLastDiscard(from)
				return true, list[i].seat, false
			}
		}
	}
	return false, -1, false
}

func simulateGame(t *testing.T, rng *rand.Rand, nPlayers int, st *simStats) {
	ids := make([]int64, nPlayers)
	names := make([]string, nPlayers)
	for i := range ids {
		ids[i] = int64(i + 1)
		names[i] = fmt.Sprintf("p%d", i)
	}
	g := NewGameWithRand(ids, names, rng.Intn(nPlayers), rng)
	seat := g.Dealer
	skipDraw := true // 庄家起手 14 张
	fromTail := false
	canDeclare := true
	kongBloom := false
	st.games++
	st.turns++

	for step := 0; step < 5000; step++ {
		st.turns++
		// ---- 摸牌 ----
		if !skipDraw {
			var aok bool
			if fromTail {
				_, aok = g.DrawReplacement(seat)
			} else {
				_, aok = g.Draw(seat)
			}
			if !aok {
				st.draws++
				if !g.Wall.Empty() {
					t.Fatalf("摸牌失败但牌墙未空")
				}
				return // 流局
			}
			kongBloom = fromTail
			canDeclare = true
			fromTail = false
		}
		skipDraw = false
		checkPerTurn(t, "摸牌后", g, seat, true)

		// ---- 本人操作(杠可连续) ----
		discardedOK := false
		for !discardedOK {
			p := g.Players[seat]
			canWin := canDeclare && g.CanSelfWin(seat)
			var kongs []KongOption
			if canDeclare {
				kongs = g.KongOptions(seat)
			}
			if canWin && rng.Intn(100) < 50 {
				st.wins++
				if kongBloom {
					st.kongBlooms++
				}
				if g.Wall.Empty() {
					st.haidi++
				}
				if !CanWin(p.Hand, len(p.Melds)) {
					t.Fatalf("CanSelfWin 与 CanWin 不一致")
				}
				return
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
					return // 墙空杠 -> 流局
				}
				kongBloom = true
				canDeclare = true
				checkPerTurn(t, "杠后补牌", g, seat, true)
				continue
			}
			tk := p.Hand[rng.Intn(len(p.Hand))]
			if !g.Discard(seat, tk) {
				t.Fatalf("Discard 失败")
			}
			discardedOK = true
		}
		kongBloom = false
		canDeclare = false
		discarded := g.Players[seat].Discards[len(g.Players[seat].Discards)-1]

		// ---- 声明结算(链式: 碰/吃后打出的牌继续结算, 与服务端一致) ----
		gangClaimed := false
		chain := 0
	claimChain:
		for {
			chain++
			if chain > 64 {
				t.Fatalf("声明链异常过长(%d)", chain)
			}
			list := buildClaims(g, seat, discarded)
			if len(list) == 0 {
				break claimChain
			}
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
			applied, who, needDraw := settleClaims(t, g, seat, discarded, list, dec, decIdx)
			if !applied {
				break claimChain
			}
			seat = who // 碰/吃/杠后轮到声明者操作
			// 从 dec 找回该座位的声明类型, 用于统计
			kind := 0
			for i := range list {
				if list[i].seat == who {
					kind = dec[i]
				}
			}
			switch kind {
			case 1:
				st.wins++
				if g.Wall.Empty() {
					st.haidi++
				}
				return
			case 2:
				st.kongMing++
				st.kongs++
				gangClaimed = true
			case 3:
				st.peng++
			case 4:
				st.chi++
			}
			if needDraw { // 明杠: 走杠后补牌流程
				gangClaimed = true
				break claimChain
			}
			if gangClaimed {
				break claimChain
			}
			// 碰/吃者出牌(canDeclare=false, 不可胡不可杠), 打出的牌继续结算
			st.chains++
			if chain > st.maxChain {
				st.maxChain = chain
			}
			p := g.Players[seat]
			tk := p.Hand[rng.Intn(len(p.Hand))]
			if !g.Discard(seat, tk) {
				t.Fatalf("出牌失败")
			}
			discarded = tk
			checkPerTurn(t, "副露后出牌", g, seat, false)
		}

		if !gangClaimed {
			seat = g.NextSeat(seat)
			skipDraw = false
			fromTail = false
			canDeclare = true
		}
		kongBloom = false
		checkInvariants(t, "回合切换", g)
	}
}

// TestSimulateFullGames 全量模拟。实测耗时约 0.3s, 但属于集成性质,
// `go test -short` 时跳过, 以满足"与普通单元测试分开运行"的要求。
func TestSimulateFullGames(t *testing.T) {
	if testing.Short() {
		t.Skip("随机全量模拟: -short 模式跳过")
	}
	rng := rand.New(rand.NewSource(20261021))
	st := &simStats{}
	for i := 0; i < 1500; i++ {
		simulateGame(t, rng, 4, st)
	}
	for i := 0; i < 500; i++ {
		simulateGame(t, rng, 3, st)
	}
	// 只断言确定性事实(局数由调用方决定); 覆盖率仅记录不断言,
	// 避免随机状态导致偶发失败(见 #77 审查意见 2)
	if st.games != 2000 {
		t.Fatalf("games=%d, want 2000", st.games)
	}
	t.Logf("统计: %d局 %d手 %d胜 %d流局", st.games, st.turns, st.wins, st.draws)
	t.Logf("杠: 总%d (暗%d 明%d 补%d) 杠上开花%d 海底%d", st.kongs, st.kongAn, st.kongMing, st.kongBu, st.kongBlooms, st.haidi)
	t.Logf("碰%d 吃%d 声明链%d次(最长%d)", st.peng, st.chi, st.chains, st.maxChain)
}

// ============================================================
// 确定性多人声明场景：验证 胡>杠>碰>吃 优先级与
// "同级按逆时针(离出牌者近)优先"的座位顺序, 与 resolveClaims 一致。
// ============================================================

// newScenarioGame 构造一个 4 人局: seat0 打出 W5, 其余各家按给定手牌设置
func newScenarioGame(t *testing.T, hands [4][]Tile) *Game {
	g := NewGameWithRand([]int64{1, 2, 3, 4}, []string{"a", "b", "c", "d"}, 0,
		rand.New(rand.NewSource(1)))
	for i := 0; i < 4; i++ {
		g.Players[i].Hand = append([]Tile(nil), hands[i]...)
	}
	// seat0 真实打出 W5(进牌河, LastSeat 指向 seat0)
	if !g.Discard(0, Tile(WanBase+4)) {
		t.Fatal("scenario: seat0 打出 W5 失败")
	}
	return g
}

func seatHand(t Tile, fillers ...string) []Tile {
	return append([]Tile{t}, tilesOf(fillers...)...)
}

func TestClaimPriorityScenarios(t *testing.T) {
	w5 := Tile(WanBase + 4)
	// 听 W5 的 13 张(各家错开花色, 避免同牌超 4 张)
	tingW5_wan := hand("W1", "W1", "W1", "W2", "W3", "W4", "W6", "W7", "T1", "T1", "T1", "T9", "T9")
	tingW5_tong := hand("O1", "O1", "O1", "O2", "O3", "O4", "W6", "W7", "P", "P", "P", "F", "F")
	// 可杠 W5(手里 3 张)
	gangW5 := hand("W5", "W5", "W5", "W1", "W2", "W3", "W4", "W6", "W7", "W8", "W9", "T1", "T2")
	// 可碰 W5(手里 2 张)
	pengW5 := hand("W5", "W5", "W1", "W2", "W3", "W4", "W6", "W7", "W8", "W9", "T1", "T2", "T3")
	// 可吃 W5(下家: W3W4+W5)
	chiW5 := hand("W3", "W4", "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5")
	// 无关 filler(不含 W5, 不可胡)
	fill := hand("W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7")

	t.Run("A_多人同时可胡_离出牌者近者优先", func(t *testing.T) {
		g := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), tingW5_wan, tingW5_tong, fill})
		list := buildClaims(g, 0, w5)
		if len(list) != 2 || list[0].seat != 1 || list[1].seat != 2 {
			t.Fatalf("候选顺序错误: %+v", list)
		}
		if !list[0].hu || !list[1].hu {
			t.Fatalf("两家都应可胡: %+v", list)
		}
		if list[0].peng || list[1].peng || list[0].gang || list[1].gang {
			t.Fatalf("不应可碰/杠: %+v", list)
		}
		// 两家都声明胡 → 离出牌者近的 seat1 胡牌
		applied, who, needDraw := settleClaims(t, g, 0, w5, list, []int{1, 1}, nil)
		if !applied || who != 1 || needDraw {
			t.Fatalf("应选 seat1 胡, got (applied=%v who=%d needDraw=%v)", applied, who, needDraw)
		}
	})

	t.Run("B_胡优先于杠", func(t *testing.T) {
		g := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), gangW5, tingW5_tong, fill})
		list := buildClaims(g, 0, w5)
		if len(list) != 2 || !list[0].gang || !list[1].hu {
			t.Fatalf("应 seat1 可杠、seat2 可胡: %+v", list)
		}
		// seat1 要杠、seat2 要胡 → 胡优先
		applied, who, _ := settleClaims(t, g, 0, w5, list, []int{2, 1}, nil)
		if !applied || who != 2 {
			t.Fatalf("胡应优先于杠, got who=%d", who)
		}
		// seat2 过 → seat1 杠成功, 且要求杠后补牌
		g2 := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), gangW5, tingW5_tong, fill})
		list2 := buildClaims(g2, 0, w5)
		applied2, who2, needDraw2 := settleClaims(t, g2, 0, w5, list2, []int{2, 0}, nil)
		if !applied2 || who2 != 1 || !needDraw2 {
			t.Fatalf("仅杠时应 seat1 明杠并需补牌, got (applied=%v who=%d needDraw=%v)", applied2, who2, needDraw2)
		}
	})

	t.Run("C_杠优先于碰", func(t *testing.T) {
		g := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), gangW5, pengW5, fill})
		list := buildClaims(g, 0, w5)
		if len(list) != 2 || !list[0].gang || !list[1].peng {
			t.Fatalf("应 seat1 可杠、seat2 可碰: %+v", list)
		}
		applied, who, _ := settleClaims(t, g, 0, w5, list, []int{2, 3}, nil)
		if !applied || who != 1 {
			t.Fatalf("杠应优先于碰, got who=%d", who)
		}
	})

	t.Run("D_碰优先于吃", func(t *testing.T) {
		g := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), chiW5, pengW5, fill})
		list := buildClaims(g, 0, w5)
		if len(list) != 2 || !list[0].peng && len(list[0].chi) == 0 {
			t.Fatalf("seat1(下家)应可吃: %+v", list)
		}
		if !list[0].hu && len(list[0].chi) == 0 {
			t.Log("seat1 可吃")
		}
		if list[1].seat != 2 || !list[1].peng {
			t.Fatalf("seat2 应可碰: %+v", list)
		}
		// seat1 要吃、seat2 要碰 → 碰优先(即使吃更近)
		applied, who, _ := settleClaims(t, g, 0, w5, list, []int{4, 3}, nil)
		if !applied || who != 2 {
			t.Fatalf("碰应优先于吃, got who=%d", who)
		}
		// 只有下家能吃: list[0] 应带吃法, 且其他座位没有吃选项
		g2 := newScenarioGame(t, [4][]Tile{seatHand(w5, "W1", "W2", "W6", "W7", "W8", "W9", "T1", "T2", "T3", "T4", "T5", "T6", "T7"), chiW5, pengW5, fill})
		list2 := buildClaims(g2, 0, w5)
		if len(list2[0].chi) == 0 {
			t.Fatal("下家 seat1 应有吃选项")
		}
		for _, c := range list2[1:] {
			if len(c.chi) != 0 {
				t.Fatalf("非下家不应有吃选项: %+v", c)
			}
		}
		// seat1 吃 → 确认吃法选择生效(C1: W3W4+W5)
		applied3, who3, _ := settleClaims(t, g2, 0, w5, list2, []int{4, 0}, []int{0, 0})
		if !applied3 || who3 != 1 {
			t.Fatalf("下家吃应生效, got (applied=%v who=%d)", applied3, who3)
		}
		if len(g2.Players[1].Melds) != 1 || g2.Players[1].Melds[0].Type != MeldChi {
			t.Fatalf("吃后应形成一组顺子副露: %+v", g2.Players[1].Melds)
		}
	})
}
