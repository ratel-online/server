package event_test

import (
	"testing"

	"github.com/feel-easy/uno/event"
	"github.com/stretchr/testify/require"
)

func TestPlayerPassed(t *testing.T) {
	listenerOne := event.NewDummyListener()
	listenerTwo := event.NewDummyListener()

	event.PlayerPassed.AddListener(listenerOne)
	event.PlayerPassed.AddListener(listenerTwo)

	payloads := []event.PlayerPassedPayload{
		{
			PlayerName: "Someone",
		},
		{
			PlayerName: "Somebody",
		},
	}

	for _, payload := range payloads {
		event.PlayerPassed.Emit(payload)
	}

	require.ElementsMatch(t, payloads, listenerOne.ReceivedPayloads())
	require.ElementsMatch(t, payloads, listenerTwo.ReceivedPayloads())
}
