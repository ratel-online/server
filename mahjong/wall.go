package mahjong

import "math/rand"

// Wall 牌墙，共 136 张。正常摸牌从头部取，杠后补牌从尾部取。
type Wall struct {
	tiles []Tile
	head  int
	tail  int
}

// NewWall 洗好一副牌
func NewWall() *Wall {
	tiles := make([]Tile, 0, NumKinds*4)
	for k := 0; k < NumKinds; k++ {
		for i := 0; i < 4; i++ {
			tiles = append(tiles, Tile(k))
		}
	}
	rand.Shuffle(len(tiles), func(i, j int) {
		tiles[i], tiles[j] = tiles[j], tiles[i]
	})
	return &Wall{tiles: tiles, head: 0, tail: len(tiles) - 1}
}

// Len 剩余张数
func (w *Wall) Len() int {
	if w == nil || w.head > w.tail {
		return 0
	}
	return w.tail - w.head + 1
}

// Empty 牌墙是否摸完
func (w *Wall) Empty() bool {
	return w.Len() == 0
}

// Draw 正常摸牌
func (w *Wall) Draw() (Tile, bool) {
	if w.Empty() {
		return 0, false
	}
	t := w.tiles[w.head]
	w.head++
	return t, true
}

// DrawTail 杠后从牌尾补牌
func (w *Wall) DrawTail() (Tile, bool) {
	if w.Empty() {
		return 0, false
	}
	t := w.tiles[w.tail]
	w.tail--
	return t, true
}
