package event

import "github.com/feel-easy/uno/card"

var CardPlayed = &cardPlayedEmitter{}

type CardPlayedPayload struct {
	PlayerName string
	Card       card.Card
}

type CardPlayedListener interface {
	OnCardPlayed(CardPlayedPayload)
}

type cardPlayedEmitter struct {
	listeners []CardPlayedListener
}

func (e *cardPlayedEmitter) AddListener(listener CardPlayedListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *cardPlayedEmitter) Emit(payload CardPlayedPayload) {
	for _, listener := range e.listeners {
		listener.OnCardPlayed(payload)
	}
}
