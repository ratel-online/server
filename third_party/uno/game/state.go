package game

import (
	"fmt"
	"strings"

	"github.com/feel-easy/uno/card"
)

type State struct {
	LastPlayedCard    card.Card
	PlayedCards       []card.Card
	CurrentPlayerHand []card.Card
	PlayerSequence    []string
	PlayerHandCounts  map[string]int
}

func (s State) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Last played card: %s", s.LastPlayedCard))

	var playerStatuses []string
	for _, playerName := range s.PlayerSequence {
		playerStatus := fmt.Sprintf("%s (%d card(s))", playerName, s.PlayerHandCounts[playerName])
		playerStatuses = append(playerStatuses, playerStatus)
	}
	lines = append(lines, fmt.Sprintf("Turn order: %s", strings.Join(playerStatuses, ", ")))
	lines = append(lines, fmt.Sprintf("Your hand: %s\n", s.CurrentPlayerHand))
	return strings.Join(lines, "\n")
}
