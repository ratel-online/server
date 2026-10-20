package game_test

import (
	"testing"

	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/game"
	"github.com/stretchr/testify/require"
)

func TestCards(t *testing.T) {
	pile := game.NewPile()
	pile.Add(card.NewNumberCard(color.Blue, 5))
	pile.Add(card.NewNumberCard(color.Green, 5))
	pile.Add(card.NewNumberCard(color.Green, 7))
	require.Equal(t, []card.Card{
		card.NewNumberCard(color.Blue, 5),
		card.NewNumberCard(color.Green, 5),
		card.NewNumberCard(color.Green, 7),
	}, pile.Cards())
}

func TestReplaceTop(t *testing.T) {
	pile := game.NewPile()
	pile.Add(card.NewNumberCard(color.Blue, 5))
	pile.Add(card.NewNumberCard(color.Green, 5))
	pile.Add(card.NewNumberCard(color.Green, 7))
	pile.Add(card.NewWildCard())
	pile.ReplaceTop(card.NewColoredCard(card.NewWildCard(), color.Yellow))
	require.Equal(t, []card.Card{
		card.NewNumberCard(color.Blue, 5),
		card.NewNumberCard(color.Green, 5),
		card.NewNumberCard(color.Green, 7),
		card.NewColoredCard(card.NewWildCard(), color.Yellow),
	}, pile.Cards())
}

func TestTop(t *testing.T) {
	pile := game.NewPile()
	require.Nil(t, pile.Top())
	pile.Add(card.NewNumberCard(color.Blue, 5))
	pile.Add(card.NewNumberCard(color.Green, 5))
	pile.Add(card.NewNumberCard(color.Green, 7))
	require.Equal(t, card.NewNumberCard(color.Green, 7), pile.Top())
}
