package game

type Pile struct {
	tiles            []int
	lastPlayer       *playerController
	originallyPlayer *playerController
}

func NewPile() *Pile {
	return &Pile{tiles: make([]int, 0, 144)}
}

func (p *Pile) SetOriginallyPlayer(player *playerController) {
	p.originallyPlayer = player
}

func (p *Pile) OriginallyPlayer() *playerController {
	return p.originallyPlayer
}

func (p *Pile) SetLastPlayer(player *playerController) {
	p.lastPlayer = player
}

func (p *Pile) LastPlayer() *playerController {
	return p.lastPlayer
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

func (d *Pile) BottomDrawOne() int {
	tile := d.tiles[len(d.tiles)-1]
	d.tiles = d.tiles[0 : len(d.tiles)-1]
	return tile
}
