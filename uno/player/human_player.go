package player

import (
	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/event"
	"github.com/ratel-online/server/uno/game"
	"github.com/ratel-online/server/uno/ui"
)

type humanPlayer struct {
	basicPlayer
}

func NewHumanPlayer(id int64, name string) game.Player {
	player := humanPlayer{basicPlayer: basicPlayer{id: id, name: name}}
	event.FirstCardPlayed.AddListener(player)
	event.CardPlayed.AddListener(player)
	event.ColorPicked.AddListener(player)
	event.PlayerPassed.AddListener(player)
	return player
}

func (p humanPlayer) PickColor(gameState game.State) color.Color {
	color := ui.PromptColor()
	return color
}

func (p humanPlayer) Play(playableCards []card.Card, gameState game.State) card.Card {
	ui.Message.HumanPlayerTurnStarted(p.name)
	ui.Println(gameState)
	card := ui.PromptCardSelection(playableCards)
	return card
}

func (p humanPlayer) OnFirstCardPlayed(payload event.FirstCardPlayedPayload) {
	ui.Message.FirstCardPlayed(payload.Card)
}

func (p humanPlayer) OnCardPlayed(payload event.CardPlayedPayload) {
	ui.Message.PlayerPlayedCard(payload.PlayerName, payload.Card)
}

func (p humanPlayer) OnColorPicked(payload event.ColorPickedPayload) {
	ui.Message.PlayerPickedColor(payload.PlayerName, payload.Color)
}

func (p humanPlayer) OnPlayerPassed(payload event.PlayerPassedPayload) {
	ui.Message.PlayerPassed(payload.PlayerName)
}

func (p humanPlayer) NotifyCardsDrawn(cards []card.Card) {
	ui.Message.HumanPlayerDrewCards(cards)
}

func (p humanPlayer) NotifyNoMatchingCardsInHand(lastPlayedCard card.Card, hand []card.Card) {
	ui.Message.HumanPlayerHasNoMatchingCardsInHand(p.name, lastPlayedCard, hand)
}
