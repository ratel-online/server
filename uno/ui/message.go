package ui

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
)

var Message = MessageWriter{}

type MessageWriter struct{}

func (m MessageWriter) FirstCardPlayed(card card.Card) {
	Printfln("First card is %s", card)
}

func (m MessageWriter) HumanPlayerDrewCards(cards []card.Card) {
	Printfln("You drew %s!", cards)
}

func (m MessageWriter) HumanPlayerHasNoMatchingCardsInHand(playerName string, lastPlayedCard card.Card, hand []card.Card) {
	Printfln("%s, none of your cards match %s!", playerName, lastPlayedCard)
	Printfln("Your hand is %s", hand)
}

func (m MessageWriter) HumanPlayerTurnStarted(playerName string) {
	Printfln("It's your turn, %s!", playerName)
}

func (m MessageWriter) PlayerDrewAndPlayedCard(playerName string, card card.Card) {
	Printfln("%s drew and played %s!", playerName, card)
}

func (m MessageWriter) PlayerDrewCards(playerName string, cards []card.Card) {
	if len(cards) == 1 {
		Printfln("%s drew a card!", playerName)
	} else {
		Printfln("%s drew %d cards!", playerName, len(cards))
	}
}

func (m MessageWriter) PlayerPassed(playerName string) {
	Printfln("%s passed!", playerName)
}

func (m MessageWriter) PlayerPickedColor(playerName string, color color.Color) {
	Printfln("%s picked color %s!", playerName, color)
}

func (m MessageWriter) PlayerPlayedCard(playerName string, card card.Card) {
	Printfln("%s played %s!", playerName, card)
}

func (m MessageWriter) PlayerTurnSkipped(playerName string) {
	Printfln("%s's turn skipped!", playerName)
}

func (m MessageWriter) TurnOrderReversed() {
	Println("Turn order has been reversed!")
}

func (m MessageWriter) Welcome() {
	Printfln(
		"WELCOME TO %s%s%s",
		color.Red.Paint("U"),
		color.Yellow.Paint("N"),
		color.Blue.Paint("O"),
	)
}

func (m MessageWriter) WinnerFound(playerName string) {
	Printfln("%s wins!", playerName)
}
