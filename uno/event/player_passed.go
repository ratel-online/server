package event

var PlayerPassed = &playerPassedEmitter{}

type PlayerPassedPayload struct {
	PlayerName string
}

type PlayerPassedListener interface {
	OnPlayerPassed(PlayerPassedPayload)
}

type playerPassedEmitter struct {
	listeners []PlayerPassedListener
}

func (e *playerPassedEmitter) AddListener(listener PlayerPassedListener) {
	e.listeners = append(e.listeners, listener)
}

func (e *playerPassedEmitter) Emit(payload PlayerPassedPayload) {
	for _, listener := range e.listeners {
		listener.OnPlayerPassed(payload)
	}
}
