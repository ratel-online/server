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
	specialPrivileges := make(map[int64][]int)
	canWin := make([]*playerController, 0)
	originallyPlayer := g.pile.originallyPlayer
	topTile := g.pile.Top()
	g.players.ForEach(func(player *playerController) {
		playerSequence = append(playerSequence, player.Name())
		playerShowCards[player.Name()] = player.GetShowCard()
		if _, ok := g.pile.SayNoPlayer()[player.ID()]; !ok &&
			topTile > 0 && g.pile.lastPlayer.ID() != player.ID() {
			if win.CanWin(append(player.Hand(), g.pile.Top()), player.GetShowCardTiles()) {
				canWin = append(canWin, player)
			}
			if card.CanGang(player.Hand(), topTile) {
				specialPrivileges[player.ID()] = append(specialPrivileges[player.ID()], consts.GANG)
			}
			if card.CanPeng(player.Hand(), topTile) {
				specialPrivileges[player.ID()] = append(specialPrivileges[player.ID()], consts.PENG)
			}
			if originallyPlayer.ID() == player.ID() &&
				card.CanChi(player.Hand(), topTile) {
				specialPrivileges[player.ID()] = append(specialPrivileges[player.ID()], consts.CHI)
			}
		}
	})
	return State{
		LastPlayer:        g.pile.lastPlayer,
		OriginallyPlayer:  originallyPlayer,
		LastPlayedTile:    g.pile.Top(),
		PlayedTiles:       g.pile.Tiles(),
		CurrentPlayerHand: player.Tiles(),
		PlayerSequence:    playerSequence,
		PlayerShowCards:   playerShowCards,
		SpecialPrivileges: specialPrivileges,
		CanWin:            canWin,
	}
}
