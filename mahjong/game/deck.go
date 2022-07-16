package game

import (
	"math/rand"

	"github.com/ratel-online/server/mahjong/tile"
)

type Deck struct {
	tiles []int
}

func NewDeck() *Deck {
	deck := &Deck{}
	fillDeck(deck)
	return deck
}

func (d *Deck) NoTiles() bool {
	return len(d.tiles) == 0
}

func (d *Deck) DrawOne() int {
	return d.Draw(1)[0]
}

func (d *Deck) Draw(amount int) []int {
	tiles := d.tiles[0:amount]
	d.tiles = d.tiles[amount:]
	return tiles
}

func (d *Deck) BottomDrawOne() int {
	tile := d.tiles[len(d.tiles)-1]
	d.tiles = d.tiles[:len(d.tiles)-1]
	return tile
}

func fillDeck(deck *Deck) {
	tiles := make([]int, 0, 144)
	generate := func(tile, num, count int) []int {
		ret := make([]int, 0, num*count)
		for i := 0; i < count; i++ {
			for j := 1; j <= num; j++ {
				ret = append(ret, tile*10+j)
			}
		}
		return ret
	}
	tiles = append(tiles, generate(tile.WAN, 9, 4)...)
	tiles = append(tiles, generate(tile.TIAO, 9, 4)...)
	tiles = append(tiles, generate(tile.BING, 9, 4)...)
	tiles = append(tiles, generate(tile.FENG, 4, 4)...)
	tiles = append(tiles, generate(tile.DRAGON, 3, 4)...)
	// tiles = append(tiles, generate(tile.SEASON, 4, 1)...)
	// tiles = append(tiles, generate(tile.HUA, 4, 1)...)
	shuffleCards(tiles)
	deck.tiles = append(deck.tiles, tiles...)
}

func shuffleCards(tiles []int) {
	rand.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] })
}
