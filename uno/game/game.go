package game

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/action"
	"github.com/ratel-online/server/uno/event"
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

func (g *Game) GetPlayerCards(name string) []card.Card {
	return g.players.GetPlayerController(name).Hand()
}

func (g *Game) DealStartingCards() {
	g.players.ForEach(func(player *playerController) {
		hand := g.deck.Draw(7)
		player.AddCards(hand)
	})
}

func (g *Game) PlayFirstCard() {
	firstCard := g.deck.DrawOne()
	g.pile.Add(firstCard)
	event.FirstCardPlayed.Emit(event.FirstCardPlayedPayload{
		Card: firstCard,
	})
	g.PerformCardActions(firstCard)
}

func (g *Game) Current() *playerController {
	return g.players.Current()
}

func (g *Game) PerformCardActions(playedCard card.Card) {
	player := g.players.Current()
	for _, cardAction := range playedCard.Actions() {
		switch cardAction := cardAction.(type) {
		case action.DrawCardsAction:
			cards := g.deck.Draw(cardAction.Amount())
			g.players.Current().AddCards(cards)
		case action.ReverseTurnsAction:
			g.players.Reverse()
		case action.SkipTurnAction:
			g.players.Skip()
		case action.PickColorAction:
			gameState := g.ExtractState(player)
			color := player.PickColor(gameState)
			coloredCard := card.NewColoredCard(playedCard, color)
			g.pile.ReplaceTop(coloredCard)
			event.ColorPicked.Emit(event.ColorPickedPayload{
				PlayerName: player.Name(),
				Color:      color,
			})
		}
	}
}

func (g Game) ExtractState(player *playerController) State {
	playerSequence := make([]string, 0)
	playerHandCounts := make(map[string]int)

	g.players.ForEach(func(player *playerController) {
		playerSequence = append(playerSequence, player.Name())
		playerHandCounts[player.Name()] = len(player.Hand())
	})

	return State{
		LastPlayedCard:    g.pile.Top(),
		PlayedCards:       g.pile.Cards(),
		CurrentPlayerHand: player.Hand(),
		PlayerSequence:    playerSequence,
		PlayerHandCounts:  playerHandCounts,
	}
}
