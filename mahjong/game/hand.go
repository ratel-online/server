package game

type Hand struct {
	tiles []int
}

func NewHand() *Hand {
	return &Hand{tiles: make([]int, 0, 17)}
}

func (h *Hand) AddTiles(tiles []int) {
	h.tiles = append(h.tiles, tiles...)
}

func (h *Hand) Tiles() []int {
	tiles := make([]int, len(h.tiles))
	copy(tiles, h.tiles)
	return tiles
}

func (h *Hand) Empty() bool {
	return len(h.tiles) == 0
}

func (h *Hand) RemoveTile(tile int) {
	for index, tileInHand := range h.tiles {
		if tileInHand == tile {
			h.tiles[index] = h.tiles[len(h.tiles)-1]
			h.tiles = h.tiles[:len(h.tiles)-1]
			return
		}
	}
}

func (h *Hand) Size() int {
	return len(h.tiles)
}
