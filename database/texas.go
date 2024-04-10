package database

import (
	"github.com/ratel-online/core/model"
)

type Texas struct {
	Room         *Room          `json:"room"`
	Players      []*TexasPlayer `json:"players"`
	Pot          uint           `json:"pot"`
	BB           int            `json:"bb"`
	SB           int            `json:"sb"`
	Pool         model.Pokers   `json:"pool"`
	Board        model.Pokers   `json:"board"`
	MaxBetAmount uint           `json:"maxBetAmount"`
	MaxBetPlayer *TexasPlayer   `json:"maxBetPlayer"`
	Round        string         `json:"round"`
	Folded       int            `json:"folded"`
	AllIn        int            `json:"allIn"`
}

func (g *Texas) Clean() {
	if g != nil {
		for _, p := range g.Players {
			close(p.State)
		}
	}
}

func (g *Texas) NextPlayer(id int64) *TexasPlayer {
	idx := -1
	for i, a := range g.Players {
		if a.ID == id {
			idx = i
		}
	}
	return g.Players[(idx+1)%len(g.Players)]
}

func (g *Texas) Player(id int64) *TexasPlayer {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (g *Texas) SBPlayer() *TexasPlayer {
	return g.Players[g.SB]
}

func (g *Texas) BBPlayer() *TexasPlayer {
	return g.Players[g.BB]
}

func (g *Texas) Bet(player *TexasPlayer, amount uint) {
	if amount > 0 {
		player.Bet(amount)
		g.Pot += amount
		if player.Amount() == 0 {
			player.AllIn = true
			g.AllIn++
		}
	}
	if g.MaxBetPlayer == nil {
		g.MaxBetPlayer = player
	}
	if player.Bets > g.MaxBetAmount {
		g.MaxBetAmount = player.Bets
		g.MaxBetPlayer = player
	}
}

type TexasPlayer struct {
	ID     int64        `json:"id"`
	Name   string       `json:"name"`
	State  chan int     `json:"state"`
	Hand   model.Pokers `json:"hand"`
	Bets   uint         `json:"bets"`
	Folded bool         `json:"folded"`
	AllIn  bool         `json:"allIn"`
}

func (p *TexasPlayer) Reset() {
	p.Bets = 0
	p.Folded = false
	p.AllIn = false
	p.Hand = nil
	p.State = make(chan int, 1)
}

func (p *TexasPlayer) Amount() uint {
	return GetPlayer(p.ID).Amount
}

func (p *TexasPlayer) Bet(amount uint) {
	p.Bets += amount
	GetPlayer(p.ID).Amount -= amount
}

func (p *TexasPlayer) Add(amount uint) {
	GetPlayer(p.ID).Amount += amount
}
