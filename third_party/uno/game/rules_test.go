package game_test

import (
	"testing"

	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/game"
	"github.com/stretchr/testify/require"
)

func TestRules(t *testing.T) {
	scenarios := []struct {
		description    string
		candidateCard  card.Card
		lastPlayedCard card.Card
		expectedResult bool
	}{
		{
			description:    "wild_card_is_always_playable",
			candidateCard:  card.NewWildCard(),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: true,
		},
		{
			description:    "wild_draw_four_card_is_always_playable",
			candidateCard:  card.NewWildDrawFourCard(),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: true,
		},
		{
			description:    "number_cards_with_same_color",
			candidateCard:  card.NewNumberCard(color.Blue, 5),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: true,
		},
		{
			description:    "number_cards_with_same_number",
			candidateCard:  card.NewNumberCard(color.Red, 7),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: true,
		},
		{
			description:    "number_cards_with_different_color_and_number",
			candidateCard:  card.NewNumberCard(color.Red, 5),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: false,
		},
		{
			description:    "reverse_cards",
			candidateCard:  card.NewReverseCard(color.Red),
			lastPlayedCard: card.NewReverseCard(color.Blue),
			expectedResult: true,
		},
		{
			description:    "skip_cards",
			candidateCard:  card.NewSkipCard(color.Red),
			lastPlayedCard: card.NewSkipCard(color.Blue),
			expectedResult: true,
		},
		{
			description:    "draw_two_cards",
			candidateCard:  card.NewDrawTwoCard(color.Red),
			lastPlayedCard: card.NewDrawTwoCard(color.Blue),
			expectedResult: true,
		},
		{
			description:    "action_cards_with_same_color",
			candidateCard:  card.NewReverseCard(color.Blue),
			lastPlayedCard: card.NewDrawTwoCard(color.Blue),
			expectedResult: true,
		},
		{
			description:    "action_cards_with_different_color",
			candidateCard:  card.NewReverseCard(color.Red),
			lastPlayedCard: card.NewDrawTwoCard(color.Blue),
			expectedResult: false,
		},
		{
			description:    "number_card_then_action_card_with_same_color",
			candidateCard:  card.NewReverseCard(color.Blue),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: true,
		},
		{
			description:    "number_card_then_action_card_with_different_color",
			candidateCard:  card.NewReverseCard(color.Red),
			lastPlayedCard: card.NewNumberCard(color.Blue, 7),
			expectedResult: false,
		},
		{
			description:    "action_card_then_number_card_with_same_color",
			candidateCard:  card.NewNumberCard(color.Blue, 7),
			lastPlayedCard: card.NewReverseCard(color.Blue),
			expectedResult: true,
		},
		{
			description:    "action_card_then_number_card_with_different_color",
			candidateCard:  card.NewNumberCard(color.Blue, 7),
			lastPlayedCard: card.NewReverseCard(color.Red),
			expectedResult: false,
		},
		{
			description:    "colored_wild_card_then_card_with_same_color",
			candidateCard:  card.NewNumberCard(color.Blue, 7),
			lastPlayedCard: card.NewColoredCard(card.NewWildCard(), color.Blue),
			expectedResult: true,
		},
		{
			description:    "colored_wild_card_then_card_with_different_color",
			candidateCard:  card.NewNumberCard(color.Red, 7),
			lastPlayedCard: card.NewColoredCard(card.NewWildCard(), color.Blue),
			expectedResult: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			result := game.Playable(scenario.candidateCard, scenario.lastPlayedCard)
			require.Equal(t, scenario.expectedResult, result)
		})
	}
}
