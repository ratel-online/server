package mahjong

import (
	"fmt"
	"sort"
	"strings"
)

// Tile 使用 0~33 表示大众麻将的 34 种牌面（每种 4 张，共 136 张，不含花牌）。
//
//	 0 ~  8：一万 ~ 九万，输入 W1 ~ W9
//	 9 ~ 17：一条 ~ 九条，输入 T1 ~ T9
//	18 ~ 26：一筒 ~ 九筒，输入 O1 ~ O9
//	27 ~ 30：东 南 西 北，输入 D N X B
//	31 ~ 33：白板 发财 红中，输入 P F Z
//
// 说明：不能用 E 表示东风——服务器把单独输入的 e 当作“退出”，
// 因此风牌统一使用拼音首字母，白板用 P（与日本麻将记法一致）。
type Tile int

const (
	// NumKinds 牌面种类数
	NumKinds = 34

	WanBase    = 0  // 万子起始
	TiaoBase   = 9  // 条子起始
	TongBase   = 18 // 筒（桶）子起始
	WindBase   = 27 // 风牌起始
	DragonBase = 31 // 箭牌起始
)

const (
	East  Tile = WindBase + iota // 东
	South                        // 南
	West                         // 西
	North                        // 北
)

const (
	White Tile = DragonBase + iota // 白板
	Green                          // 发财
	Red                            // 红中
)

var (
	rankNames = []string{"一", "二", "三", "四", "五", "六", "七", "八", "九"}
	suitNames = []string{"万", "条", "筒"}
	honorName = []string{"东", "南", "西", "北", "白", "发", "中"}
)

func (t Tile) Valid() bool {
	return t >= 0 && t < NumKinds
}

// IsHonor 是否为字牌（风牌 + 箭牌）
func (t Tile) IsHonor() bool {
	return t >= WindBase
}

// IsSuited 是否为数牌（万 / 条 / 筒）
func (t Tile) IsSuited() bool {
	return t >= 0 && t < WindBase
}

// Suit 返回花色：0 万，1 条，2 筒，3 字牌
func (t Tile) Suit() int {
	if t < 0 {
		return 3
	}
	if t >= WindBase {
		return 3
	}
	return int(t) / 9
}

// Rank 返回点数：数牌 1~9，字牌 1~7
func (t Tile) Rank() int {
	if t < WindBase {
		return int(t)%9 + 1
	}
	return int(t) - WindBase + 1
}

// Code 返回便于输入的牌码，如 W1 / T9 / O5 / E / Z
func (t Tile) Code() string {
	switch {
	case t >= 0 && t < TiaoBase:
		return fmt.Sprintf("W%d", t+1)
	case t >= TiaoBase && t < TongBase:
		return fmt.Sprintf("T%d", t-TiaoBase+1)
	case t >= TongBase && t < WindBase:
		return fmt.Sprintf("O%d", t-TongBase+1)
	}
	switch t {
	case East:
		return "D"
	case South:
		return "N"
	case West:
		return "X"
	case North:
		return "B"
	case White:
		return "P"
	case Green:
		return "F"
	case Red:
		return "Z"
	}
	return "??"
}

// Name 返回中文牌名，如 一万 / 五条 / 红中
func (t Tile) Name() string {
	if !t.Valid() {
		return "??"
	}
	if t < WindBase {
		return rankNames[t.Rank()-1] + suitNames[t.Suit()]
	}
	return honorName[t-WindBase]
}

func (t Tile) String() string {
	return t.Code()
}

var (
	codeMap = map[string]Tile{}
	nameMap = map[string]Tile{}
)

func init() {
	for k := 0; k < NumKinds; k++ {
		t := Tile(k)
		codeMap[t.Code()] = t
		nameMap[t.Name()] = t
	}
	nameMap["萬"] = WanBase
	for k := 0; k < 9; k++ {
		nameMap[rankNames[k]+"萬"] = Tile(WanBase + k)
		nameMap[rankNames[k]+"條"] = Tile(TiaoBase + k)
		nameMap[rankNames[k]+"饼"] = Tile(TongBase + k)
		nameMap[rankNames[k]+"餅"] = Tile(TongBase + k)
	}
	// 常见别名
	nameMap["白板"] = White
	nameMap["发财"] = Green
	nameMap["紅中"] = Red
	nameMap["红中"] = Red
	codeMap["BB"] = White
}

// ParseTile 解析单张牌。
// 支持 W1 / T9 / O5 / E / S / X / N / B / F / Z，
// 也支持 1W / 3w / 5万 / 一万 / 东 等写法。
func ParseTile(s string) (Tile, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	if t, ok := codeMap[s]; ok {
		return t, true
	}
	if t, ok := nameMap[s]; ok {
		return t, true
	}
	var digits, suffix string
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits += string(r)
		} else {
			suffix += string(r)
		}
	}
	if len(digits) != 1 {
		return 0, false
	}
	rank := int(digits[0] - '0')
	if rank < 1 || rank > 9 {
		return 0, false
	}
	switch suffix {
	case "W", "万", "萬":
		return Tile(WanBase + rank - 1), true
	case "T", "条", "條":
		return Tile(TiaoBase + rank - 1), true
	case "O", "筒", "桶", "饼", "餅":
		return Tile(TongBase + rank - 1), true
	}
	return 0, false
}

// ParseTiles 解析多张牌，支持 "W1 W2"、"W1,W2"、"W1W2"、"一万二万" 等写法。
func ParseTiles(s string) ([]Tile, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	rs := []rune(s)
	out := make([]Tile, 0, 14)
	for i := 0; i < len(rs); {
		r := rs[i]
		if r == ' ' || r == ',' || r == '+' || r == '|' || r == '/' || r == '\t' {
			i++
			continue
		}
		matched := false
		for width := 3; width >= 1; width-- {
			if i+width > len(rs) {
				continue
			}
			if t, ok := ParseTile(string(rs[i : i+width])); ok {
				out = append(out, t)
				i += width
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("无法识别的牌：%q", string(r))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有可识别的牌")
	}
	return out, nil
}

// SortTiles 按 万→条→筒→风→箭 的顺序排序
func SortTiles(tiles []Tile) {
	sort.Slice(tiles, func(i, j int) bool { return tiles[i] < tiles[j] })
}

// FormatTiles 分组展示手牌，如 "W1 W2 W3 | T5 T5 | E Z"
func FormatTiles(tiles []Tile) string {
	if len(tiles) == 0 {
		return "-"
	}
	sorted := append([]Tile(nil), tiles...)
	SortTiles(sorted)
	groups := make([]string, 4)
	for _, t := range sorted {
		s := t.Suit()
		if s < 0 || s > 3 {
			continue
		}
		if groups[s] != "" {
			groups[s] += " "
		}
		groups[s] += t.Code()
	}
	out := make([]string, 0, 4)
	for _, g := range groups {
		if g != "" {
			out = append(out, g)
		}
	}
	return strings.Join(out, " | ")
}
