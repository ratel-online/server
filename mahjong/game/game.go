package game

import (
	"github.com/ratel-online/server/mahjong/card"
	"github.com/ratel-online/server/mahjong/consts"
	"github.com/ratel-online/server/mahjong/tile"
	"github.com/ratel-online/server/mahjong/win"
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
		if len(g.pile.Tiles()) > 0 && g.pile.lastPlayer.ID() != player.ID() {
			if card.CanGang(player.Hand(), g.pile.Top()) {
				specialPrivileges[player.ID()] = consts.GANG
			}
			if card.CanPeng(player.Hand(), g.pile.Top()) {
				specialPrivileges[player.ID()] = consts.PENG
			}
			if win.CanWin(append(player.Hand(), g.pile.Top()), player.GetShowCardTiles()) {
				specialPrivileges[player.ID()] = consts.WIN
			}
		}
	})
	playedTiles := g.pile.Tiles()
	tiles := player.Tiles()
	return State{
		LastPlayer:        g.pile.lastPlayer,
		LastPlayedTile:    g.pile.Top(),
		PlayedTiles:       playedTiles,
		CurrentPlayerHand: tiles,
		PlayerSequence:    playerSequence,
		PlayerShowCards:   playerShowCards,
		SpecialPrivileges: specialPrivileges,
	}
}
