package database

import (
	"bytes"
	"fmt"
)

// NiuniuGame 斗牛游戏数据
type NiuniuGame struct {
	Room           *Room                       `json:"room"`
	Players        []int64                     `json:"players"`        // 玩家ID列表
	PlayerData     map[int64]*NiuniuPlayerData `json:"playerData"`     // 玩家数据
	States         map[int64]chan int          `json:"states"`         // 玩家状态通道
	Bets           map[int64]int               `json:"bets"`           // 玩家下注分数
	BetReady       int                         `json:"betReady"`       // 已下注人数
	ShowReady      int                         `json:"showReady"`      // 已亮牌人数
	LowIncomeCount map[int64]int               `json:"lowIncomeCount"` // 低保领取次数
}

// Clean 清理游戏资源
func (game *NiuniuGame) Clean() {
	if game != nil {
		for _, state := range game.States {
			close(state)
		}
	}
}

// Card 扑克牌结构
type Card struct {
	Suit  int `json:"suit"`  // 花色 (0=方块, 1=梅花, 2=红桃, 3=黑桃)
	Point int `json:"point"` // 点数 (1=A, 11=J, 12=Q, 13=K)
}

// GetValue 获取牌面分数(10,J,Q,K都算10分)
func (c Card) GetValue() int {
	if c.Point >= 10 {
		return 10
	}
	return c.Point
}

// String 牌的字符串表示
func (c Card) String() string {
	suits := []string{"♦", "♣", "♥", "♠"}
	points := []string{"", "A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
	return fmt.Sprintf("%s%s", suits[c.Suit], points[c.Point])
}

// NiuniuPlayerData 斗牛玩家数据
type NiuniuPlayerData struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Cards []Card `json:"cards"`
	Score int    `json:"score"`
}

// NewNiuniuPlayer 创建斗牛玩家
func NewNiuniuPlayer(user *Player) *NiuniuPlayerData {
	return &NiuniuPlayerData{
		ID:    user.ID,
		Name:  user.Name,
		Cards: make([]Card, 0, 5),
		Score: 0,
	}
}

// PlayerID 获取玩家ID
func (p *NiuniuPlayerData) PlayerID() int {
	return int(p.ID)
}

// NickName 获取玩家昵称
func (p *NiuniuPlayerData) NickName() string {
	return p.Name
}

// ShowCards 显示玩家手牌
func (p *NiuniuPlayerData) ShowCards() string {
	buf := bytes.Buffer{}
	buf.WriteString("[")
	for i, card := range p.Cards {
		if i > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(card.String())
	}
	buf.WriteString("]")
	return buf.String()
}

// CardType 牌型常量
const (
	CardTypeNone       = iota // 无分
	CardTypeHasPoint          // 有分 (牛1-牛9)
	CardTypeNiuniu            // 牛牛
	CardTypeFourFlower        // 四花
	CardTypeFiveFlower        // 五花
	CardTypeFiveSmall         // 五小
	CardTypeBomb              // 炸弹
)

// Suit 花色常量
const (
	SuitDiamond = 0 // 方块
	SuitClub    = 1 // 梅花
	SuitHeart   = 2 // 红桃
	SuitSpade   = 3 // 黑桃
)

// CardTypeResult 牌型结果
type CardTypeResult struct {
	Type    int  `json:"type"`    // 牌型类型
	Point   int  `json:"point"`   // 点数(牛几)
	MaxCard Card `json:"maxCard"` // 最大单牌
}

// AnalyzeCards 分析牌型
func (p *NiuniuPlayerData) AnalyzeCards() CardTypeResult {
	if len(p.Cards) != 5 {
		return CardTypeResult{Type: CardTypeNone}
	}

	// 检查炸弹
	if p.isBomb() {
		return CardTypeResult{
			Type:    CardTypeBomb,
			MaxCard: p.getMaxCard(),
		}
	}

	// 检查五小
	if p.isFiveSmall() {
		return CardTypeResult{
			Type:    CardTypeFiveSmall,
			MaxCard: p.getMaxCard(),
		}
	}

	// 检查五花
	if p.isFiveFlower() {
		return CardTypeResult{
			Type:    CardTypeFiveFlower,
			MaxCard: p.getMaxCard(),
		}
	}

	// 检查四花
	if p.isFourFlower() {
		return CardTypeResult{
			Type:    CardTypeFourFlower,
			MaxCard: p.getMaxCard(),
		}
	}

	// 检查牛牛或有分
	point, hasNiu := p.calculateNiu()
	maxCard := p.getMaxCard()

	if !hasNiu {
		return CardTypeResult{
			Type:    CardTypeNone,
			MaxCard: maxCard,
		}
	}

	if point == 0 {
		return CardTypeResult{
			Type:    CardTypeNiuniu,
			MaxCard: maxCard,
		}
	}

	return CardTypeResult{
		Type:    CardTypeHasPoint,
		Point:   point,
		MaxCard: maxCard,
	}
}

// isBomb 是否是炸弹(4张同点数)
func (p *NiuniuPlayerData) isBomb() bool {
	pointCount := make(map[int]int)
	for _, c := range p.Cards {
		pointCount[c.Point]++
		if pointCount[c.Point] == 4 {
			return true
		}
	}
	return false
}

// isFiveSmall 是否是五小(5张牌点数都<5且和为10)
func (p *NiuniuPlayerData) isFiveSmall() bool {
	sum := 0
	for _, c := range p.Cards {
		if c.Point >= 5 {
			return false
		}
		sum += c.Point
	}
	return sum == 10
}

// isFiveFlower 是否是五花(5张都是J,Q,K)
func (p *NiuniuPlayerData) isFiveFlower() bool {
	for _, c := range p.Cards {
		if c.Point < 11 {
			return false
		}
	}
	return true
}

// isFourFlower 是否是四花(1张10,4张J,Q,K)
func (p *NiuniuPlayerData) isFourFlower() bool {
	flowerCount := 0
	hasTen := false
	for _, c := range p.Cards {
		if c.Point >= 11 {
			flowerCount++
		} else if c.Point == 10 {
			hasTen = true
		}
	}
	return flowerCount == 4 && hasTen
}

// calculateNiu 计算牛几(返回点数和是否有牛)
func (p *NiuniuPlayerData) calculateNiu() (int, bool) {
	cards := p.Cards
	// 尝试所有3张牌的组合
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 4; j++ {
			for k := j + 1; k < 5; k++ {
				sum := cards[i].GetValue() + cards[j].GetValue() + cards[k].GetValue()
				if sum%10 == 0 {
					// 找到能组成10的倍数的3张牌
					remaining := 0
					for idx, c := range cards {
						if idx != i && idx != j && idx != k {
							remaining += c.GetValue()
						}
					}
					point := remaining % 10
					return point, true
				}
			}
		}
	}
	return 0, false
}

// getMaxCard 获取最大的牌
func (p *NiuniuPlayerData) getMaxCard() Card {
	maxCard := p.Cards[0]
	for _, c := range p.Cards[1:] {
		if compareCard(c, maxCard) > 0 {
			maxCard = c
		}
	}
	return maxCard
}

// compareCard 比较两张牌大小(先比点数,再比花色)
func compareCard(c1, c2 Card) int {
	if c1.Point != c2.Point {
		return c1.Point - c2.Point
	}
	return c1.Suit - c2.Suit
}

// GetCardTypeName 获取牌型名称
func GetCardTypeName(ct CardTypeResult) string {
	switch ct.Type {
	case CardTypeBomb:
		return "Bomb"
	case CardTypeFiveSmall:
		return "Five Small"
	case CardTypeFiveFlower:
		return "Five Flower"
	case CardTypeFourFlower:
		return "Four Flower"
	case CardTypeNiuniu:
		return "NiuNiu"
	case CardTypeHasPoint:
		return fmt.Sprintf("牛%d", ct.Point)
	default:
		return "没牛"
	}
}

// CompareCardType 比较两个牌型大小 (返回1表示ct1大,-1表示ct2大,0表示相等)
func CompareCardType(ct1, ct2 CardTypeResult) int {
	// 先比牌型
	if ct1.Type != ct2.Type {
		return ct1.Type - ct2.Type
	}

	// 同牌型比较
	switch ct1.Type {
	case CardTypeHasPoint:
		if ct1.Point != ct2.Point {
			return ct1.Point - ct2.Point
		}
		return compareCard(ct1.MaxCard, ct2.MaxCard)
	default:
		return compareCard(ct1.MaxCard, ct2.MaxCard)
	}
}

// GetCardTypeScore 根据牌型获取基础分数
func GetCardTypeScore(ct CardTypeResult) int {
	switch ct.Type {
	case CardTypeBomb:
		return 20
	case CardTypeFiveSmall, CardTypeFiveFlower:
		return 25
	case CardTypeFourFlower, CardTypeNiuniu:
		return 15
	case CardTypeHasPoint:
		if ct.Point >= 8 {
			return 10
		}
		return 5
	default:
		return 5
	}
}
