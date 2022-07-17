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

func (l *DummyListener) OnCardPlayed(payload CardPlayedPayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}

func (l *DummyListener) OnFirstCardPlayed(payload FirstCardPlayedPayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}

func (l *DummyListener) OnColorPicked(payload ColorPickedPayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}

func (l *DummyListener) OnPlayerPassed(payload PlayerPassedPayload) {
	l.receivedPayloads = append(l.receivedPayloads, payload)
}
