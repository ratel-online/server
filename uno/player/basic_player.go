package player

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/ui"
)

type basicPlayer struct {
	name string
}

func (p basicPlayer) Name() string {
	return p.name
}

func (p basicPlayer) NotifyCardsDrawn(cards []card.Card) {
	ui.Message.PlayerDrewCards(p.name, cards)
}

func (p basicPlayer) NotifyNoMatchingCardsInHand(lastPlayedCard card.Card, hand []card.Card) {
}
