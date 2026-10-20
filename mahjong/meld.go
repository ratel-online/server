package mahjong

import "strings"

// MeldType 副露类型
type MeldType int

const (
	MeldChi      MeldType = iota + 1 // 吃
	MeldPeng                         // 碰
	MeldGangMing                     // 明杠（杠别人打出的牌）
	MeldGangAn                       // 暗杠
	MeldGangBu                       // 补杠（碰过之后再杠）
)

func (t MeldType) String() string {
	switch t {
	case MeldChi:
		return "吃"
	case MeldPeng:
		return "碰"
	case MeldGangMing:
		return "明杠"
	case MeldGangAn:
		return "暗杠"
	case MeldGangBu:
		return "补杠"
	}
	return "副露"
}

// Meld 一组已亮出的牌。顺子只记录最小的一张，刻子/杠记录牌面本身。
type Meld struct {
	Type MeldType `json:"type"`
	Tile Tile     `json:"tile"`
	From int      `json:"from"` // 牌的来源座位，-1 表示自己摸到
}

// Tiles 展开这组牌
func (m Meld) Tiles() []Tile {
	switch m.Type {
	case MeldChi:
		return []Tile{m.Tile, m.Tile + 1, m.Tile + 2}
	case MeldPeng:
		return []Tile{m.Tile, m.Tile, m.Tile}
	case MeldGangMing, MeldGangAn, MeldGangBu:
		return []Tile{m.Tile, m.Tile, m.Tile, m.Tile}
	}
	return nil
}

// IsGang 是否为杠
func (m Meld) IsGang() bool {
	return m.Type == MeldGangMing || m.Type == MeldGangAn || m.Type == MeldGangBu
}

func (m Meld) String() string {
	return m.Type.String() + " " + FormatTiles(m.Tiles())
}

// FormatMelds 展示所有副露
func FormatMelds(melds []Meld) string {
	if len(melds) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(melds))
	for _, m := range melds {
		parts = append(parts, "["+m.String()+"]")
	}
	return strings.Join(parts, " ")
}
