package rule

import (
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
)

// 够级规则（山东够级简化版）：
//   - 6 人 2 副牌，牌点大小 3<4<5<6<7<8<9<10<J<Q<K<A<2<小王<大王。
//   - 牌型只有两种：同点数 N 张（1~8 张），以及"挂王"（同点数 N 张 + 若干张同种王）。
//   - 挂王牌只能被挂王牌压：王大者大，王相同比主体点数。
//   - 普通牌只能被同张数、点数更大的普通牌压。
//   - 够级牌：含王、含 2、五个10、四个J、三个Q、两个K、两个A。
//     够级牌只有对门可以接（四户乱残后除外），对门不要则其余玩家也不能接。

const (
	// GoujiValSmallJoker 小王在够级里的点数值
	GoujiValSmallJoker = 16
	// GoujiValBigJoker 大王在够级里的点数值
	GoujiValBigJoker = 17
)

// GoujiValue 够级点数值：key 为 core poker 的牌键（1=A, 2=2, 3~10=3~10, 11=J, 12=Q, 13=K, 14=小王, 15=大王）。
func GoujiValue(key int) int {
	switch key {
	case 1: // A
		return 14
	case 2: // 2
		return 15
	case 14: // 小王
		return GoujiValSmallJoker
	case 15: // 大王
		return GoujiValBigJoker
	}
	return key
}

// GoujiHand 一手够级牌。
// 全部为王的手牌视作"主体为王"的普通牌型（例如两张小王 = 主体小王 2 张）。
type GoujiHand struct {
	MainVal    int  // 主体点数值
	MainCount  int  // 主体张数
	JokerVal   int  // 挂王的点数值，0 表示不挂王
	JokerCount int  // 挂王的张数
	Gouji      bool // 是否够级牌
	Total      int  // 总张数
}

// ParseGoujiHand 把选出的牌解析成够级牌型，结构非法返回 nil：
//   - 主体点数只允许一种（够级没有顺子、连对等组合）；
//   - 挂王只允许一种王（大小王不能混挂）。
func ParseGoujiHand(cards modelx.Pokers) *GoujiHand {
	if len(cards) == 0 {
		return nil
	}
	mainCount := map[int]int{}
	sc, xc := 0, 0
	for _, card := range cards {
		if card.Key < 1 || card.Key > 15 {
			return nil
		}
		switch card.Key {
		case 14:
			sc++
		case 15:
			xc++
		default:
			mainCount[GoujiValue(card.Key)]++
		}
	}
	if len(mainCount) > 1 || (sc > 0 && xc > 0) {
		return nil
	}
	hand := &GoujiHand{Total: len(cards)}
	if len(mainCount) == 1 {
		for v, c := range mainCount {
			hand.MainVal, hand.MainCount = v, c
		}
	}
	hasJoker := sc+xc > 0
	switch {
	case sc > 0:
		hand.JokerVal, hand.JokerCount = GoujiValSmallJoker, sc
	case xc > 0:
		hand.JokerVal, hand.JokerCount = GoujiValBigJoker, xc
	}
	if hand.MainVal == 0 || hand.MainCount == 0 {
		// 全王手牌：主体就是王本身，归一化成普通牌型
		if hand.JokerVal == 0 {
			return nil
		}
		hand.MainVal, hand.MainCount = hand.JokerVal, hand.JokerCount
		hand.JokerVal, hand.JokerCount = 0, 0
	}
	hand.Gouji = hasJoker || hand.isGouji()
	return hand
}

// isGouji 够级牌判定：含王、含 2、五个10、四个J、三个Q、两个K、两个A。
func (h *GoujiHand) isGouji() bool {
	if h.JokerCount > 0 || h.MainVal == 15 {
		return true
	}
	switch h.MainVal {
	case 10:
		return h.MainCount >= 5
	case 11:
		return h.MainCount >= 4
	case 12:
		return h.MainCount >= 3
	case 13, 14: // K、A 各两张即为够级牌
		return h.MainCount >= 2
	}
	return false
}

// CanBeat 判断 h 是否能压过 last。张数必须相同；挂王牌只能被挂王牌压。
func (h *GoujiHand) CanBeat(last *GoujiHand) bool {
	if h == nil || last == nil || h.Total != last.Total {
		return false
	}
	if last.JokerCount > 0 {
		if h.JokerCount == 0 || h.MainCount != last.MainCount {
			return false
		}
		if h.JokerVal != last.JokerVal {
			return h.JokerVal > last.JokerVal
		}
		return h.MainVal > last.MainVal
	}
	if h.JokerCount > 0 {
		return false
	}
	return h.MainVal > last.MainVal
}

// CanBeatAny 判断手牌里是否存在至少一手能压过 last 的牌。
// 用于自动过牌与提示：压不过的玩家不再询问，直接跳过。
func CanBeatAny(hand modelx.Pokers, last *GoujiHand) bool {
	if last == nil {
		return true
	}
	total := last.Total
	counts := map[int]int{}
	sc, xc := 0, 0
	for _, card := range hand {
		switch card.Key {
		case 14:
			sc++
		case 15:
			xc++
		default:
			counts[GoujiValue(card.Key)]++
		}
	}
	// 普通（不挂王）候选：同点数凑够 total 张
	if last.JokerCount == 0 {
		for v, c := range counts {
			if c >= total {
				cand := &GoujiHand{MainVal: v, MainCount: total, Total: total}
				cand.Gouji = cand.isGouji()
				if cand.CanBeat(last) {
					return true
				}
			}
		}
		// 全王候选：同种王凑够 total 张
		for _, jc := range []int{sc, xc} {
			if jc >= total {
				jv := GoujiValSmallJoker
				if xc >= total {
					jv = GoujiValBigJoker
				}
				cand := &GoujiHand{MainVal: jv, MainCount: total, Total: total}
				cand.Gouji = cand.isGouji()
				if cand.CanBeat(last) {
					return true
				}
			}
		}
		return false
	}
	// 挂王候选：主体 last.MainCount 张 + 王 last.JokerCount 张
	for v, c := range counts {
		if c < last.MainCount {
			continue
		}
		for jv, jcHave := range map[int]int{GoujiValSmallJoker: sc, GoujiValBigJoker: xc} {
			if jcHave < last.JokerCount {
				continue
			}
			cand := &GoujiHand{MainVal: v, MainCount: last.MainCount, JokerVal: jv, JokerCount: last.JokerCount, Total: total}
			cand.Gouji = cand.isGouji()
			if cand.CanBeat(last) {
				return true
			}
		}
	}
	return false
}

// GoujiHandString 把一手牌转成输入风格的字符串（用于广播，如 "444s"）。
func GoujiHandString(cards modelx.Pokers) string {
	s := ""
	for _, card := range cards {
		s += poker.GetAlias(card.Key)
	}
	return s
}
