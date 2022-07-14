package game

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/ui"
)

type playerController struct {
	player Player
	hand   *Hand
}

func newPlayerController(player Player) *playerController {
	return &playerController{
		player: player,
		hand:   NewHand(),
	}
}

func (c *playerController) AddCards(cards []card.Card) {
	c.hand.AddCards(cards)
	c.player.NotifyCardsDrawn(cards)
}

func (c *playerController) Hand() []card.Card {
	return c.hand.Cards()
}

func (c *playerController) Name() string {
	return c.player.Name()
}

func (c *playerController) NoCards() bool {
	return c.hand.Empty()
}

func (c *playerController) PickColor(gameState State) color.Color {
	return c.player.PickColor(gameState)
}

func (c *playerController) Play(gameState State, deck *Deck) card.Card {
	playableCards := c.hand.PlayableCards(gameState.LastPlayedCard)
	if len(playableCards) == 0 {
		c.player.NotifyNoMatchingCardsInHand(gameState.LastPlayedCard, gameState.CurrentPlayerHand)
		playableDrawnCard := c.tryTopDecking(gameState, deck)
		return playableDrawnCard
	}

	for {
		selectedCard := c.player.Play(playableCards, gameState)
		if !contains(playableCards, selectedCard) {
			ui.Printfln("Cheat detected! Card %s is not in %s's hand!", selectedCard, c.player.Name())
			continue
		}
		c.hand.RemoveCard(selectedCard)
		return selectedCard
	}
}

func (c *playerController) tryTopDecking(gameState State, deck *Deck) card.Card {
	extraCard := deck.DrawOne()
	c.AddCards([]card.Card{extraCard})
	if Playable(extraCard, gameState.LastPlayedCard) {
		c.hand.RemoveCard(extraCard)
		return extraCard
	}
	return nil
}

func contains(cards []card.Card, searchedCard card.Card) bool {
	for _, card := range cards {
		if card.Equal(searchedCard) {
			return true
		}
	}
	return false
}
