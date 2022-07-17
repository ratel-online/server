package event

type DummyListener struct {
	receivedPayloads []interface{}
}

func NewDummyListener() *DummyListener {
	return &DummyListener{receivedPayloads: make([]interface{}, 0)}
}

func (l *DummyListener) ReceivedPayloads() []interface{} {
	return l.receivedPayloads
}

func (l *DummyListener) OnTilePlayed(payload TilePlayedPayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}

func (l *DummyListener) OnPlayTile(payload PlayTilePayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}
