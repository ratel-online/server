package game

import (
	"github.com/feel-easy/uno/card"
)

func Playable(candidateCard card.Card, lastPlayedCard card.Card) bool {
	if candidateCard.Color() == lastPlayedCard.Color() {
		return true
	}

	switch candidateCard := candidateCard.(type) {
	case card.WildCard, card.WildDrawFourCard:
		return true
	case card.DrawTwoCard:
		_, isDrawTwoCard := lastPlayedCard.(card.DrawTwoCard)
		return isDrawTwoCard
	case card.ReverseCard:
		_, isReverseCard := lastPlayedCard.(card.ReverseCard)
		return isReverseCard
	case card.SkipCard:
		_, isSkipCard := lastPlayedCard.(card.SkipCard)
		return isSkipCard
	case card.NumberCard:
		lastPlayedCard, isNumberCard := lastPlayedCard.(card.NumberCard)
		return isNumberCard && lastPlayedCard.Number() == candidateCard.Number()
	default:
		return false
	}
}
