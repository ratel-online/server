package event

var TilePlayed = &tilePlayedEmitter{}

type TilePlayedPayload struct {
	PlayerName string
	Tile       int
}

type TilePlayedListener interface {
	OnTilePlayed(TilePlayedPayload)
}

type tilePlayedEmitter struct {
	listeners []TilePlayedListener
}

func (e *tilePlayedEmitter) AddListener(listener TilePlayedListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *tilePlayedEmitter) Emit(payload TilePlayedPayload) {
	for _, listener := range e.listeners {
		listener.OnTilePlayed(payload)
	}
}
