package event

import "github.com/feel-easy/uno/card"

var FirstCardPlayed = &firstCardPlayedEmitter{}

type FirstCardPlayedPayload struct {
	Card card.Card
}

type FirstCardPlayedListener interface {
	OnFirstCardPlayed(FirstCardPlayedPayload)
}

type firstCardPlayedEmitter struct {
	listeners []FirstCardPlayedListener
}

func (e *firstCardPlayedEmitter) AddListener(listener FirstCardPlayedListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *firstCardPlayedEmitter) Emit(payload FirstCardPlayedPayload) {
	for _, listener := range e.listeners {
		listener.OnFirstCardPlayed(payload)
	}
}
