package game

type Pile struct {
	tiles []int
}

func NewPile() *Pile {
	return &Pile{tiles: make([]int, 0, 144)}
}

func (p *Pile) Add(tile int) {
	p.tiles = append(p.tiles, tile)
}

func (p *Pile) Tiles() []int {
	tiles := make([]int, len(p.tiles))
	copy(tiles, p.tiles)
	return tiles
}

func (p *Pile) ReplaceTop(tile int) {
	p.tiles[len(p.tiles)-1] = tile
}

func (p *Pile) Top() int {
	pileSize := len(p.tiles)
	if pileSize == 0 {
		return 0
	}
	return p.tiles[pileSize-1]
}
