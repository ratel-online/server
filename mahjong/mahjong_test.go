package mahjong

import (
	"strings"
	"testing"
)

func mustTiles(t *testing.T, s string) []Tile {
	t.Helper()
	tiles, err := ParseTiles(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tiles
}

func TestParseTile(t *testing.T) {
	cases := map[string]Tile{
		"W1": 0, "w9": 8, "T5": 13, "O1": 18, "o9": 26,
		"D": East, "N": South, "X": West, "B": North,
		"P": White, "F": Green, "Z": Red, "BB": White,
		"5万": Tile(4), "一万": Tile(0), "3条": Tile(11), "9筒": Tile(26),
	}
	for in, want := range cases {
		got, ok := ParseTile(in)
		if !ok || got != want {
			t.Fatalf("ParseTile(%q) = %v,%v want %v", in, got, ok, want)
		}
	}
	if _, ok := ParseTile("W0"); ok {
		t.Fatal("W0 should be invalid")
	}
}

func TestParseTiles(t *testing.T) {
	tiles := mustTiles(t, "W1 W2W3,T4")
	if len(tiles) != 4 || tiles[0] != 0 || tiles[3] != 12 {
		t.Fatalf("unexpected tiles: %v", tiles)
	}
}

func TestCanWin(t *testing.T) {
	cases := []struct {
		hand  string
		melds int
		want  bool
	}{
		{"W1 W2 W3 W4 W5 W6 T1 T2 T3 O1 O2 O3 D D", 0, true},
		{"W1 W1 W1 W2 W3 W4 T5 T6 T7 O9 O9 O9 Z Z", 0, true},
		{"W1 W2 W3 W4 W5 W6 T1 T2 T3 O1 O2 O3 D F", 0, false},
		// 七对
		{"W1 W1 W2 W2 W3 W3 T4 T4 T5 T5 O6 O6 D D", 0, true},
		// 豪华七对（四张相同算两对）
		{"W1 W1 W1 W1 W2 W2 T4 T4 T5 T5 O6 O6 D D", 0, true},
		// 十三幺
		{"W1 W9 T1 T9 O1 O9 D N X B P F Z Z", 0, true},
		// 带一副副露
		{"W1 W2 W3 T1 T2 T3 O1 O2 O3 D D", 1, true},
		{"W1 W2 W4 T1 T2 T3 O1 O2 O3 D D", 1, false},
	}
	for _, c := range cases {
		got := CanWin(mustTiles(t, c.hand), c.melds)
		if got != c.want {
			t.Fatalf("CanWin(%q, %d) = %v want %v", c.hand, c.melds, got, c.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	// 清一色 + 碰碰胡
	hand := mustTiles(t, "W1 W1 W1 W2 W2 W2 W3 W3 W3 W4 W4")
	melds := []Meld{{Type: MeldPeng, Tile: Tile(4), From: 1}}
	s := Evaluate(hand, melds, WinContext{SelfDraw: true})
	if s.Fan < 6 {
		t.Fatalf("expect at least 6 fan, got %d %v", s.Fan, s.Items)
	}
}

func TestChiOptions(t *testing.T) {
	g := NewGame([]int64{1, 2, 3}, []string{"a", "b", "c"}, 0)
	p := g.Players[1]
	p.Hand = mustTiles(t, "W1 W2 W4 W5 T3 O7 D")
	opts := g.ChiOptions(1, Tile(2)) // 三万：W1W2 / W2W4 / W4W5
	if len(opts) != 3 {
		t.Fatalf("expect 3 chi options, got %v", opts)
	}
	if len(g.ChiOptions(1, East)) != 0 {
		t.Fatal("cannot chi honor tiles")
	}
}

func newTestGame(t *testing.T) *Game {
	t.Helper()
	g := NewGame([]int64{1, 2, 3}, []string{"a", "b", "c"}, 0)
	for _, p := range g.Players {
		p.Hand = nil
		p.Melds = nil
		p.Discards = nil
	}
	return g
}

func TestTenpaiAndWinWith(t *testing.T) {
	g := newTestGame(t)
	g.Players[0].Hand = mustTiles(t, "W1 W2 W3 W4 W5 W6 T1 T2 T3 O1 O2 O3 D")
	// 单钓东风
	if g.CanWinWith(0, East) != true {
		t.Fatal("should win with east")
	}
	if g.CanWinWith(0, South) {
		t.Fatal("should not win with south")
	}
	g.Players[0].AddTile(East)
	if !g.CanSelfWin(0) {
		t.Fatal("should be self-drawn win")
	}
	s := Evaluate(g.Players[0].Hand, nil, WinContext{SelfDraw: true})
	if s.Fan < 2 || !strings.Contains(strings.Join(s.Items, "·"), "自摸") {
		t.Fatalf("expect 平胡+自摸 2 fan, got %+v", s)
	}
}

func TestKongFlow(t *testing.T) {
	g := newTestGame(t)
	p := g.Players[0]
	p.Hand = mustTiles(t, "W1 W1 W1 W1 T2 T3 T4 O5 O6 O7 D D Z")

	opts := g.KongOptions(0)
	if len(opts) != 1 || opts[0].Tile != Tile(0) || opts[0].Kind != KongAn {
		t.Fatalf("expect concealed kong W1, got %+v", opts)
	}
	if !g.ConcealedKong(0, Tile(0)) {
		t.Fatal("concealed kong failed")
	}
	if len(p.Hand) != 9 || len(p.Melds) != 1 || !p.Melds[0].IsGang() {
		t.Fatalf("unexpected state after kong: hand=%v melds=%v", p.Hand, p.Melds)
	}

	// 碰过之后可以补杠
	p2 := g.Players[1]
	p2.Melds = []Meld{{Type: MeldPeng, Tile: Tile(9), From: 0}}
	p2.Hand = mustTiles(t, "T1 T1 T2 T3 O5 O6 O7 D D Z")
	opts = g.KongOptions(1)
	if len(opts) != 1 || opts[0].Kind != KongBu {
		t.Fatalf("expect added kong, got %+v", opts)
	}
	if !g.AddedKong(1, Tile(9)) || p2.Melds[0].Type != MeldGangBu {
		t.Fatal("added kong failed")
	}
}

func TestClaims(t *testing.T) {
	g := newTestGame(t)
	p := g.Players[1]
	p.Hand = mustTiles(t, "W3 W3 W4 W5 T1 T2 T3 O1 O2 O3 D D Z")
	g.LastSeat = 0

	if len(g.ChiOptions(1, Tile(2))) != 1 {
		t.Fatalf("expect 1 chi option, got %v", g.ChiOptions(1, Tile(2)))
	}
	if !g.ClaimChi(1, Tile(2), [2]Tile{Tile(3), Tile(4)}) {
		t.Fatal("chi failed")
	}
	if len(p.Melds) != 1 || p.Melds[0].Tile != Tile(2) {
		t.Fatalf("unexpected meld: %+v", p.Melds)
	}

	p.Hand = mustTiles(t, "W5 W5 W5 T1 T2 T3 O1 O2 O3 D D Z")
	if !g.CanPeng(1, Tile(4)) || !g.CanGangFromDiscard(1, Tile(4)) {
		t.Fatal("should be able to peng and gang W5")
	}
	if !g.ClaimGang(1, Tile(4)) {
		t.Fatal("gang failed")
	}
	if p.CountTile(Tile(4)) != 0 || len(p.Melds) != 2 {
		t.Fatalf("unexpected state after gang: %v %v", p.Hand, p.Melds)
	}
}

func TestPointsPerFan(t *testing.T) {
	if PointsPerFan != 100 {
		t.Fatalf("1 番应为 100 积分，got %d", PointsPerFan)
	}
	cases := []struct {
		hand string
		ctx  WinContext
		fan  int
	}{
		{"W1 W2 W3 W4 W5 W6 T1 T2 T3 O1 O2 O3 D D", WinContext{}, 2},               // 平胡 + 门前清
		{"W1 W2 W3 W4 W5 W6 T1 T2 T3 O1 O2 O3 D D", WinContext{SelfDraw: true}, 2}, // 平胡 + 自摸
		{"W1 W2 W3 W4 W5 W6 W7 W8 W9 W1 W2 W3 W4 W4", WinContext{}, 6},             // 平胡 + 清一色 + 门前清
	}
	for _, c := range cases {
		s := Evaluate(mustTiles(t, c.hand), nil, c.ctx)
		if s.Fan != c.fan {
			t.Fatalf("%s expect %d fan, got %d %v", c.hand, c.fan, s.Fan, s.Items)
		}
		if s.Points != c.fan*PointsPerFan {
			t.Fatalf("%s expect %d points, got %d", c.hand, c.fan*PointsPerFan, s.Points)
		}
	}
}
