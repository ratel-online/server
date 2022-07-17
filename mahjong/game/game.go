package game

import (
	"sort"

	"github.com/ratel-online/server/mahjong/card"
	"github.com/ratel-online/server/mahjong/consts"
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

func (g *Game) GetPlayerTiles(id int64) string {
	tiles := g.players.GetPlayerController(id).Hand()
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
	playerShowCards := make(map[string][]*ShowCard)
	specialPrivileges := make(map[int64]int)
	g.players.ForEach(func(player *playerController) {
		playerSequence = append(playerSequence, player.Name())
		playerShowCards[player.Name()] = player.GetShowCard()
		if card.CanGang(player.Hand(), g.pile.Top()) {
			specialPrivileges[player.ID()] = consts.GANG
		}
		if card.CanPeng(player.Hand(), g.pile.Top()) {
			specialPrivileges[player.ID()] = consts.PENG
		}
	})
	playedTiles := g.pile.Tiles()
	sort.Ints(playedTiles)
	return State{
		LastPlayedTile:    g.pile.Top(),
		PlayedTiles:       playedTiles,
		CurrentPlayerHand: player.Hand(),
		PlayerSequence:    playerSequence,
		PlayerShowCards:   playerShowCards,
		SpecialPrivileges: specialPrivileges,
	}
}
