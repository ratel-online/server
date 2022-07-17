package event

var PlayTile = &playTileEmitter{}

type PlayTilePayload struct {
	PlayerName string
	Tile       int
}

type PlayTileListener interface {
	OnPlayTile(PlayTilePayload)
}

type playTileEmitter struct {
	listeners []PlayTileListener
}

func (e *playTileEmitter) AddListener(listener PlayTileListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *playTileEmitter) Emit(payload PlayTilePayload) {
	for _, listener := range e.listeners {
		listener.OnPlayTile(payload)
	}
}
