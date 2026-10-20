package event

import "github.com/feel-easy/uno/card/color"

var ColorPicked = &colorPickedEmitter{}

type ColorPickedPayload struct {
	PlayerName string
	Color      color.Color
}

type ColorPickedListener interface {
	OnColorPicked(ColorPickedPayload)
}

type colorPickedEmitter struct {
	listeners []ColorPickedListener
}

func (e *colorPickedEmitter) AddListener(listener ColorPickedListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *colorPickedEmitter) Emit(payload ColorPickedPayload) {
	for _, listener := range e.listeners {
		listener.OnColorPicked(payload)
	}
}
