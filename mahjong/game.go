package mahjong

import (
	"fmt"
	"math/rand"
	"time"
)

// Player 牌桌上的一个座位
type Player struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Hand     []Tile `json:"hand"`
	Melds    []Meld `json:"melds"`
	Discards []Tile `json:"discards"`
}

// AddTile 摸牌入手（保持有序）
func (p *Player) AddTile(t Tile) {
	p.Hand = append(p.Hand, t)
	SortTiles(p.Hand)
}

// RemoveTile 打出一张牌
func (p *Player) RemoveTile(t Tile) bool {
	for i, v := range p.Hand {
		if v == t {
			p.Hand = append(p.Hand[:i], p.Hand[i+1:]...)
			return true
		}
	}
	return false
}

// HasTile 手上是否有这张牌
func (p *Player) HasTile(t Tile) bool {
	for _, v := range p.Hand {
		if v == t {
			return true
		}
	}
	return false
}

// CountTile 手上这张牌的张数
func (p *Player) CountTile(t Tile) int {
	n := 0
	for _, v := range p.Hand {
		if v == t {
			n++
		}
	}
	return n
}

func (p *Player) String() string {
	return p.Name
}

// KongKind 自己回合的杠牌类型
type KongKind int

const (
	KongAn KongKind = iota + 1 // 暗杠
	KongBu                     // 补杠
)

func (k KongKind) String() string {
	if k == KongAn {
		return "暗杠"
	}
	return "补杠"
}

// KongOption 一个可执行的杠
type KongOption struct {
	Tile Tile
	Kind KongKind
}

// Game 一局大众麻将
type Game struct {
	Players  []*Player
	Wall     *Wall
	Dealer   int
	Turn     int
	LastTile Tile
	LastSeat int
}

// NewGame 创建并洗牌发牌（庄家 14 张，其余 13 张）
func NewGame(ids []int64, names []string, dealer int) *Game {
	return NewGameWithRand(ids, names, dealer, rand.New(rand.NewSource(time.Now().UnixNano())))
}

// NewGameWithRand 用指定随机源创建牌局（便于测试注入确定性随机序列）
func NewGameWithRand(ids []int64, names []string, dealer int, r *rand.Rand) *Game {
	players := make([]*Player, 0, len(ids))
	for i, id := range ids {
		name := ""
		if i < len(names) {
			name = names[i]
		}
		if name == "" {
			name = fmt.Sprintf("玩家%d", id)
		}
		players = append(players, &Player{ID: id, Name: name})
	}
	if dealer < 0 || dealer >= len(players) {
		dealer = 0
	}
	g := &Game{Players: players, Wall: NewWallWithRand(r), Dealer: dealer, Turn: dealer, LastTile: -1, LastSeat: -1}
	for round := 0; round < 13; round++ {
		for _, p := range players {
			if t, ok := g.Wall.Draw(); ok {
				p.AddTile(t)
			}
		}
	}
	if t, ok := g.Wall.Draw(); ok {
		players[dealer].AddTile(t)
	}
	return g
}

// SeatOf 按玩家 ID 找座位，找不到返回 -1
func (g *Game) SeatOf(id int64) int {
	for i, p := range g.Players {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// Current 当前该操作的座位
func (g *Game) Current() *Player {
	if g.Turn < 0 || g.Turn >= len(g.Players) {
		return nil
	}
	return g.Players[g.Turn]
}

// NextSeat 下家
func (g *Game) NextSeat(seat int) int {
	return (seat + 1) % len(g.Players)
}

// PrevSeat 上家
func (g *Game) PrevSeat(seat int) int {
	return (seat + len(g.Players) - 1) % len(g.Players)
}

// Draw 正常摸牌
func (g *Game) Draw(seat int) (Tile, bool) {
	if seat < 0 || seat >= len(g.Players) {
		return 0, false
	}
	t, ok := g.Wall.Draw()
	if !ok {
		return 0, false
	}
	g.Players[seat].AddTile(t)
	return t, true
}

// DrawReplacement 杠后补牌
func (g *Game) DrawReplacement(seat int) (Tile, bool) {
	if seat < 0 || seat >= len(g.Players) {
		return 0, false
	}
	t, ok := g.Wall.DrawTail()
	if !ok {
		return 0, false
	}
	g.Players[seat].AddTile(t)
	return t, true
}

// Discard 打出指定牌
func (g *Game) Discard(seat int, t Tile) bool {
	p := g.Players[seat]
	if !p.RemoveTile(t) {
		return false
	}
	p.Discards = append(p.Discards, t)
	g.LastTile = t
	g.LastSeat = seat
	return true
}

// RemoveLastDiscard 弃牌被吃碰杠后从牌河取走
func (g *Game) RemoveLastDiscard(seat int) {
	if seat < 0 || seat >= len(g.Players) {
		return
	}
	p := g.Players[seat]
	if len(p.Discards) > 0 {
		p.Discards = p.Discards[:len(p.Discards)-1]
	}
}

// CanSelfWin 当前手牌（14 张形态）是否已经和牌
func (g *Game) CanSelfWin(seat int) bool {
	p := g.Players[seat]
	return CanWin(p.Hand, len(p.Melds))
}

// CanWinWith 别人打出/补杠的牌能否和
func (g *Game) CanWinWith(seat int, t Tile) bool {
	p := g.Players[seat]
	hand := append([]Tile(nil), p.Hand...)
	hand = append(hand, t)
	return CanWin(hand, len(p.Melds))
}

// ChiOptions 可以吃这张牌的三种组合（仅下家可吃，且只能吃数牌）
func (g *Game) ChiOptions(seat int, t Tile) [][2]Tile {
	if !t.IsSuited() {
		return nil
	}
	p := g.Players[seat]
	rank := t.Rank()
	var opts [][2]Tile
	if rank >= 3 && t-2 >= 0 && p.HasTile(t-2) && p.HasTile(t-1) {
		opts = append(opts, [2]Tile{t - 2, t - 1})
	}
	if rank >= 2 && rank <= 8 && p.HasTile(t-1) && p.HasTile(t+1) {
		opts = append(opts, [2]Tile{t - 1, t + 1})
	}
	if rank <= 7 && p.HasTile(t+1) && p.HasTile(t+2) {
		opts = append(opts, [2]Tile{t + 1, t + 2})
	}
	return opts
}

// CanPeng 是否可以碰
func (g *Game) CanPeng(seat int, t Tile) bool {
	return g.Players[seat].CountTile(t) >= 2
}

// CanGangFromDiscard 是否可以明杠
func (g *Game) CanGangFromDiscard(seat int, t Tile) bool {
	return g.Players[seat].CountTile(t) >= 3
}

// KongOptions 自己回合可以执行的暗杠 / 补杠
func (g *Game) KongOptions(seat int) []KongOption {
	p := g.Players[seat]
	opts := make([]KongOption, 0, 4)
	seen := map[Tile]bool{}
	for _, t := range p.Hand {
		if seen[t] {
			continue
		}
		seen[t] = true
		if p.CountTile(t) == 4 {
			opts = append(opts, KongOption{Tile: t, Kind: KongAn})
		}
	}
	for _, m := range p.Melds {
		if m.Type == MeldPeng && p.HasTile(m.Tile) {
			opts = append(opts, KongOption{Tile: m.Tile, Kind: KongBu})
		}
	}
	return opts
}

// ConcealedKong 暗杠
func (g *Game) ConcealedKong(seat int, t Tile) bool {
	p := g.Players[seat]
	if p.CountTile(t) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		p.RemoveTile(t)
	}
	p.Melds = append(p.Melds, Meld{Type: MeldGangAn, Tile: t, From: -1})
	g.Turn = seat
	return true
}

// AddedKong 补杠
func (g *Game) AddedKong(seat int, t Tile) bool {
	p := g.Players[seat]
	if !p.HasTile(t) {
		return false
	}
	for i, m := range p.Melds {
		if m.Type == MeldPeng && m.Tile == t {
			p.RemoveTile(t)
			p.Melds[i] = Meld{Type: MeldGangBu, Tile: t, From: m.From}
			g.Turn = seat
			return true
		}
	}
	return false
}

// ClaimGang 明杠别人打出的牌
func (g *Game) ClaimGang(seat int, t Tile) bool {
	p := g.Players[seat]
	if p.CountTile(t) < 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		p.RemoveTile(t)
	}
	p.Melds = append(p.Melds, Meld{Type: MeldGangMing, Tile: t, From: g.LastSeat})
	g.Turn = seat
	return true
}

// ClaimPeng 碰别人打出的牌
func (g *Game) ClaimPeng(seat int, t Tile) bool {
	p := g.Players[seat]
	if p.CountTile(t) < 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		p.RemoveTile(t)
	}
	p.Melds = append(p.Melds, Meld{Type: MeldPeng, Tile: t, From: g.LastSeat})
	g.Turn = seat
	return true
}

// ClaimChi 吃别人打出的牌，pair 为手上用于组合的两张
func (g *Game) ClaimChi(seat int, t Tile, pair [2]Tile) bool {
	p := g.Players[seat]
	if !p.HasTile(pair[0]) || !p.HasTile(pair[1]) {
		return false
	}
	p.RemoveTile(pair[0])
	p.RemoveTile(pair[1])
	base := t
	for _, v := range pair {
		if v < base {
			base = v
		}
	}
	p.Melds = append(p.Melds, Meld{Type: MeldChi, Tile: base, From: g.LastSeat})
	g.Turn = seat
	return true
}
