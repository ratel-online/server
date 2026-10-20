package mahjong

// 本文件负责和牌判定与番型计算。
//
// 大众麻将（推倒胡）的基本牌型：4 组面子（顺子/刻子）+ 1 对将牌，
// 另外支持七对、豪华七对与十三幺。

func countsOf(tiles []Tile) [NumKinds]int {
	var c [NumKinds]int
	for _, t := range tiles {
		if t.Valid() {
			c[t]++
		}
	}
	return c
}

func countSum(c *[NumKinds]int) int {
	sum := 0
	for _, v := range c {
		sum += v
	}
	return sum
}

// CanWin 判断手牌（暗牌）在已有 meldCount 组副露的情况下是否和牌。
func CanWin(hand []Tile, meldCount int) bool {
	need := 14 - meldCount*3
	if need < 2 || len(hand) != need {
		return false
	}
	c := countsOf(hand)
	if meldCount == 0 {
		if isSevenPairs(c) || isThirteenOrphans(c) {
			return true
		}
	}
	// 依次尝试每一张做将牌
	for i := 0; i < NumKinds; i++ {
		if c[i] < 2 {
			continue
		}
		c[i] -= 2
		ok := decompose(&c, (need-2)/3)
		c[i] += 2
		if ok {
			return true
		}
	}
	return false
}

// decompose 从最小的牌开始拆面子
func decompose(c *[NumKinds]int, melds int) bool {
	if melds == 0 {
		return countSum(c) == 0
	}
	for i := 0; i < NumKinds; i++ {
		if c[i] == 0 {
			continue
		}
		// 刻子
		if c[i] >= 3 {
			c[i] -= 3
			ok := decompose(c, melds-1)
			c[i] += 3
			if ok {
				return true
			}
		}
		// 顺子（仅数牌，且不跨越花色）
		if i < WindBase && i%9 <= 6 && c[i+1] > 0 && c[i+2] > 0 {
			c[i]--
			c[i+1]--
			c[i+2]--
			ok := decompose(c, melds-1)
			c[i]++
			c[i+1]++
			c[i+2]++
			if ok {
				return true
			}
		}
		// 最小的这张牌无法组成面子，剪枝
		return false
	}
	return false
}

// isSevenPairs 七对（4 张相同算两对）
func isSevenPairs(c [NumKinds]int) bool {
	pairs := 0
	for _, v := range c {
		if v%2 != 0 {
			return false
		}
		pairs += v / 2
	}
	return pairs == 7
}

// isThirteenOrphans 十三幺：全部幺九牌与字牌各一张，其中一对作将
func isThirteenOrphans(c [NumKinds]int) bool {
	if countSum(&c) != 14 {
		return false
	}
	orphans := []Tile{0, 8, 9, 17, 18, 26, East, South, West, North, White, Green, Red}
	pair := 0
	for _, t := range orphans {
		switch c[t] {
		case 0:
			return false
		case 1:
		default:
			pair += c[t] - 1
		}
	}
	total := 0
	for _, v := range c {
		total += v
	}
	return total == 14 && pair == 1
}

// isAllTriplets 判断暗牌能否全部拆成刻子 + 一对将
func isAllTriplets(c [NumKinds]int) bool {
	for i := 0; i < NumKinds; i++ {
		if c[i] < 2 {
			continue
		}
		c[i] -= 2
		ok := true
		for j := 0; j < NumKinds; j++ {
			if c[j]%3 != 0 {
				ok = false
				break
			}
		}
		c[i] += 2
		if ok {
			return true
		}
	}
	return false
}

// WinContext 和牌时的情景
type WinContext struct {
	SelfDraw  bool // 自摸
	KongBloom bool // 杠上开花
	RobKong   bool // 抢杠胡
	LastTile  bool // 海底捞月
}

// PointsPerFan 一番兑换的积分（与房间积分系统对接，1 番 = 100 积分）
const PointsPerFan = 100

// Score 番型与得分
type Score struct {
	Fan    int
	Items  []string
	Points int // 应付积分 = Fan * PointsPerFan
}

// Evaluate 计算番型。hand 必须包含和牌的那张牌。
func Evaluate(hand []Tile, melds []Meld, ctx WinContext) Score {
	var s Score
	c := countsOf(hand)
	all := append([]Tile(nil), hand...)
	for _, m := range melds {
		all = append(all, m.Tiles()...)
	}

	special := false
	if len(melds) == 0 && isThirteenOrphans(c) {
		s.Items = append(s.Items, "十三幺")
		s.Fan += 13
		special = true
	} else if len(melds) == 0 && isSevenPairs(c) {
		if hasQuad(c) {
			s.Items = append(s.Items, "豪华七对")
			s.Fan += 8
		} else {
			s.Items = append(s.Items, "七对")
			s.Fan += 4
		}
		special = true
	}

	if !special {
		// 普通和牌都含 1 番底（平胡），七对 / 十三幺本身已含底番
		s.Items = append(s.Items, "平胡")
		s.Fan++
	}

	if !special && isAllTripletsConcealed(c, melds) {
		s.Items = append(s.Items, "碰碰胡")
		s.Fan += 2
	}

	if isFlush(all) {
		s.Items = append(s.Items, "清一色")
		s.Fan += 4
	}

	if isConcealed(melds) && !ctx.SelfDraw {
		s.Items = append(s.Items, "门前清")
		s.Fan++
	}
	if ctx.SelfDraw {
		s.Items = append(s.Items, "自摸")
		s.Fan++
	}
	if ctx.KongBloom {
		s.Items = append(s.Items, "杠上开花")
		s.Fan++
	}
	if ctx.RobKong {
		s.Items = append(s.Items, "抢杠胡")
		s.Fan++
	}
	if ctx.LastTile {
		s.Items = append(s.Items, "海底捞月")
		s.Fan++
	}

	// 积分线性结算：1 番 = 100 积分
	s.Points = s.Fan * PointsPerFan
	return s
}

func hasQuad(c [NumKinds]int) bool {
	for _, v := range c {
		if v == 4 {
			return true
		}
	}
	return false
}

func isAllTripletsConcealed(c [NumKinds]int, melds []Meld) bool {
	for _, m := range melds {
		if m.Type == MeldChi {
			return false
		}
	}
	return isAllTriplets(c)
}

func isFlush(tiles []Tile) bool {
	if len(tiles) == 0 {
		return false
	}
	suit := tiles[0].Suit()
	if suit > 2 {
		return false
	}
	for _, t := range tiles {
		if t.Suit() != suit {
			return false
		}
	}
	return true
}

func isConcealed(melds []Meld) bool {
	for _, m := range melds {
		switch m.Type {
		case MeldChi, MeldPeng, MeldGangMing:
			return false
		}
	}
	return true
}
