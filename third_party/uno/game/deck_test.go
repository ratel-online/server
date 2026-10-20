package game_test

import (
	"testing"

	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/game"
	"github.com/stretchr/testify/require"
)

func TestDraw(t *testing.T) {
	t.Run("returns_all_108_standard_uno_cards", func(t *testing.T) {
		deck := game.NewDeck()
		cards := deck.Draw(108)
		require.ElementsMatch(t, standardDeckCards, cards)
	})

	t.Run("returns_no_cards_when_argument_is_zero", func(t *testing.T) {
		deck := game.NewDeck()
		cards := deck.Draw(0)
		require.Empty(t, cards)
	})

	t.Run("refills_itself_upon_becoming_empty", func(t *testing.T) {
		deck := game.NewDeck()

		cards := make([]card.Card, 0, 216)
		cards = append(cards, deck.Draw(50)...)
		cards = append(cards, deck.Draw(50)...)
		cards = append(cards, deck.Draw(50)...)
		cards = append(cards, deck.Draw(50)...)
		cards = append(cards, deck.Draw(16)...)

		doubledStandardDeckCards := make([]card.Card, 0, 216)
		doubledStandardDeckCards = append(doubledStandardDeckCards, standardDeckCards...)
		doubledStandardDeckCards = append(doubledStandardDeckCards, standardDeckCards...)
		require.ElementsMatch(t, doubledStandardDeckCards, cards)
	})
}

func TestDrawOne(t *testing.T) {
	deck := game.NewDeck()
	card := deck.DrawOne()
	require.Contains(t, standardDeckCards, card)
}

var standardDeckCards = []card.Card{
	card.NewWildCard(),
	card.NewWildCard(),
	card.NewWildCard(),
	card.NewWildCard(),
	card.NewWildDrawFourCard(),
	card.NewWildDrawFourCard(),
	card.NewWildDrawFourCard(),
	card.NewWildDrawFourCard(),
	card.NewDrawTwoCard(color.Blue),
	card.NewDrawTwoCard(color.Blue),
	card.NewReverseCard(color.Blue),
	card.NewReverseCard(color.Blue),
	card.NewSkipCard(color.Blue),
	card.NewSkipCard(color.Blue),
	card.NewNumberCard(color.Blue, 0),
	card.NewNumberCard(color.Blue, 1),
	card.NewNumberCard(color.Blue, 1),
	card.NewNumberCard(color.Blue, 2),
	card.NewNumberCard(color.Blue, 2),
	card.NewNumberCard(color.Blue, 3),
	card.NewNumberCard(color.Blue, 3),
	card.NewNumberCard(color.Blue, 4),
	card.NewNumberCard(color.Blue, 4),
	card.NewNumberCard(color.Blue, 5),
	card.NewNumberCard(color.Blue, 5),
	card.NewNumberCard(color.Blue, 6),
	card.NewNumberCard(color.Blue, 6),
	card.NewNumberCard(color.Blue, 7),
	card.NewNumberCard(color.Blue, 7),
	card.NewNumberCard(color.Blue, 8),
	card.NewNumberCard(color.Blue, 8),
	card.NewNumberCard(color.Blue, 9),
	card.NewNumberCard(color.Blue, 9),
	card.NewDrawTwoCard(color.Green),
	card.NewDrawTwoCard(color.Green),
	card.NewReverseCard(color.Green),
	card.NewReverseCard(color.Green),
	card.NewSkipCard(color.Green),
	card.NewSkipCard(color.Green),
	card.NewNumberCard(color.Green, 0),
	card.NewNumberCard(color.Green, 1),
	card.NewNumberCard(color.Green, 1),
	card.NewNumberCard(color.Green, 2),
	card.NewNumberCard(color.Green, 2),
	card.NewNumberCard(color.Green, 3),
	card.NewNumberCard(color.Green, 3),
	card.NewNumberCard(color.Green, 4),
	card.NewNumberCard(color.Green, 4),
	card.NewNumberCard(color.Green, 5),
	card.NewNumberCard(color.Green, 5),
	card.NewNumberCard(color.Green, 6),
	card.NewNumberCard(color.Green, 6),
	card.NewNumberCard(color.Green, 7),
	card.NewNumberCard(color.Green, 7),
	card.NewNumberCard(color.Green, 8),
	card.NewNumberCard(color.Green, 8),
	card.NewNumberCard(color.Green, 9),
	card.NewNumberCard(color.Green, 9),
	card.NewDrawTwoCard(color.Red),
	card.NewDrawTwoCard(color.Red),
	card.NewReverseCard(color.Red),
	card.NewReverseCard(color.Red),
	card.NewSkipCard(color.Red),
	card.NewSkipCard(color.Red),
	card.NewNumberCard(color.Red, 0),
	card.NewNumberCard(color.Red, 1),
	card.NewNumberCard(color.Red, 1),
	card.NewNumberCard(color.Red, 2),
	card.NewNumberCard(color.Red, 2),
	card.NewNumberCard(color.Red, 3),
	card.NewNumberCard(color.Red, 3),
	card.NewNumberCard(color.Red, 4),
	card.NewNumberCard(color.Red, 4),
	card.NewNumberCard(color.Red, 5),
	card.NewNumberCard(color.Red, 5),
	card.NewNumberCard(color.Red, 6),
	card.NewNumberCard(color.Red, 6),
	card.NewNumberCard(color.Red, 7),
	card.NewNumberCard(color.Red, 7),
	card.NewNumberCard(color.Red, 8),
	card.NewNumberCard(color.Red, 8),
	card.NewNumberCard(color.Red, 9),
	card.NewNumberCard(color.Red, 9),
	card.NewDrawTwoCard(color.Yellow),
	card.NewDrawTwoCard(color.Yellow),
	card.NewReverseCard(color.Yellow),
	card.NewReverseCard(color.Yellow),
	card.NewSkipCard(color.Yellow),
	card.NewSkipCard(color.Yellow),
	card.NewNumberCard(color.Yellow, 0),
	card.NewNumberCard(color.Yellow, 1),
	card.NewNumberCard(color.Yellow, 1),
	card.NewNumberCard(color.Yellow, 2),
	card.NewNumberCard(color.Yellow, 2),
	card.NewNumberCard(color.Yellow, 3),
	card.NewNumberCard(color.Yellow, 3),
	card.NewNumberCard(color.Yellow, 4),
	card.NewNumberCard(color.Yellow, 4),
	card.NewNumberCard(color.Yellow, 5),
	card.NewNumberCard(color.Yellow, 5),
	card.NewNumberCard(color.Yellow, 6),
	card.NewNumberCard(color.Yellow, 6),
	card.NewNumberCard(color.Yellow, 7),
	card.NewNumberCard(color.Yellow, 7),
	card.NewNumberCard(color.Yellow, 8),
	card.NewNumberCard(color.Yellow, 8),
	card.NewNumberCard(color.Yellow, 9),
	card.NewNumberCard(color.Yellow, 9),
}
