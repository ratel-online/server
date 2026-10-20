package game

import (
	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
)

type Player interface {
	PlayerID() int
	NickName() string
	PickColor(gameState State) color.Color
	Play(playableCards []card.Card, gameState State) (card.Card, error)
	NotifyCardsDrawn(drawnCards []card.Card)
	NotifyNoMatchingCardsInHand(lastPlayedCard card.Card, hand []card.Card)
}
