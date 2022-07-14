package game

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/action"
	"github.com/ratel-online/server/uno/event"
)

type Game struct {
	players *playerIterator
	deck    *Deck
	pile    *Pile
}

func New(players []Player) *Game {
	return &Game{
		players: newPlayerIterator(players),
		deck:    NewDeck(),
		pile:    NewPile(),
	}
}

func (g *Game) Play() Player {
	g.dealStartingCards()
	g.playFirstCard()

	for {
		player := g.players.Next()
		gameState := g.extractState(player)
		card := player.Play(gameState, g.deck)
		if card == nil {
			event.PlayerPassed.Emit(event.PlayerPassedPayload{
				PlayerName: player.Name(),
			})
			continue
		}
		g.pile.Add(card)
		event.CardPlayed.Emit(event.CardPlayedPayload{
			PlayerName: player.Name(),
			Card:       card,
		})
		g.performCardActions(card)
		if player.NoCards() {
			return player.player
		}
	}
}

func (g *Game) dealStartingCards() {
	g.players.ForEach(func(player *playerController) {
		hand := g.deck.Draw(7)
		player.AddCards(hand)
	})
}

func (g *Game) playFirstCard() {
	firstCard := g.deck.DrawOne()
	g.pile.Add(firstCard)
	event.FirstCardPlayed.Emit(event.FirstCardPlayedPayload{
		Card: firstCard,
	})
	g.performCardActions(firstCard)
}

func (g *Game) performCardActions(playedCard card.Card) {
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
			gameState := g.extractState(player)
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

func (g Game) extractState(player *playerController) State {
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
