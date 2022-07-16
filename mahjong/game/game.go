package game

import (
	"sort"

	"github.com/ratel-online/server/mahjong/tile"
)

type Game struct {
	players *PlayerIterator
	deck    *Deck
	pile    *Pile
}

func (g *Game) Players() *PlayerIterator {
	return g.players
}

func (g *Game) Deck() *Deck {
	return g.deck
}

func (g *Game) Pile() *Pile {
	return g.pile
}

func New(players []Player) *Game {
	return &Game{
		players: newPlayerIterator(players),
		deck:    NewDeck(),
		pile:    NewPile(),
	}
}

func (g *Game) GetPlayerTiles(name string) string {
	tiles := g.players.GetPlayerController(name).Hand()
	return tile.ToTileString(tiles)
}

func (g *Game) DealStartingTiles() {
	g.players.ForEach(func(player *playerController) {
		hand := g.deck.Draw(13)
		player.AddTiles(hand)
	})
}

func (g *Game) Current() *playerController {
	return g.players.Current()
}

func (g Game) ExtractState(player *playerController) State {
	playerSequence := make([]string, 0)
	playerHandCounts := make(map[string]int)

	g.players.ForEach(func(player *playerController) {
		playerSequence = append(playerSequence, player.Name())
		playerHandCounts[player.Name()] = len(player.Hand())
	})
	playedTiles := g.pile.Tiles()
	sort.Ints(playedTiles)
	return State{
		LastPlayedTile:    g.pile.Top(),
		PlayedTiles:       playedTiles,
		CurrentPlayerHand: player.Hand(),
		PlayerSequence:    playerSequence,
	}
}
