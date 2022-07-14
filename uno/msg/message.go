package msg

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
)

var Message = MessageWriter{}

type MessageWriter struct{}

func (m MessageWriter) FirstCardPlayed(card card.Card) string {
	return Sprintfln("First card is %s", card)
}

func (m MessageWriter) HumanPlayerDrewCards(cards []card.Card) string {
	return Sprintfln("You drew %s!", cards)
}

func (m MessageWriter) HumanPlayerHasNoMatchingCardsInHand(playerName string, lastPlayedCard card.Card, hand []card.Card) string {
	return Sprintfln("%s, none of your cards match %s!", playerName, lastPlayedCard)
	return Sprintfln("Your hand is %s", hand)
}

func (m MessageWriter) HumanPlayerTurnStarted(playerName string) string {
	return Sprintfln("It's your turn, %s!", playerName)
}

func (m MessageWriter) PlayerDrewAndPlayedCard(playerName string, card card.Card) string {
	return Sprintfln("%s drew and played %s!", playerName, card)
}

func (m MessageWriter) PlayerDrewCards(playerName string, cards []card.Card) string {
	if len(cards) == 1 {
		return Sprintfln("%s drew a card!", playerName)
	} else {
		return Sprintfln("%s drew %d cards!", playerName, len(cards))
	}
}

func (m MessageWriter) PlayerPassed(playerName string) string {
	return Sprintfln("%s passed!", playerName)
}

func (m MessageWriter) PlayerPickedColor(playerName string, color color.Color) string {
	return Sprintfln("%s picked color %s!", playerName, color)
}

func (m MessageWriter) PlayerPlayedCard(playerName string, card card.Card) string {
	return Sprintfln("%s played %s!", playerName, card)
}

func (m MessageWriter) PlayerTurnSkipped(playerName string) string {
	return Sprintfln("%s's turn skipped!", playerName)
}

func (m MessageWriter) TurnOrderReversed() string {
	return Sprintln("Turn order has been reversed!")
}

func (m MessageWriter) Welcome() string {
	return Sprintfln(
		"WELCOME TO %s%s%s",
		color.Red.Paint("U"),
		color.Yellow.Paint("N"),
		color.Blue.Paint("O"),
	)
}

func (m MessageWriter) WinnerFound(playerName string) string {
	return Sprintfln("%s wins!", playerName)
}
