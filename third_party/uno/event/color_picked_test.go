package event_test

import (
	"testing"

	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/event"
	"github.com/stretchr/testify/require"
)

func TestColorPicked(t *testing.T) {
	listenerOne := event.NewDummyListener()
	listenerTwo := event.NewDummyListener()

	event.ColorPicked.AddListener(listenerOne)
	event.ColorPicked.AddListener(listenerTwo)

	payloads := []event.ColorPickedPayload{
		{
			PlayerName: "Someone",
			Color:      color.Red,
		},
		{
			PlayerName: "Somebody",
			Color:      color.Yellow,
		},
	}

	for _, payload := range payloads {
		event.ColorPicked.Emit(payload)
	}

	require.ElementsMatch(t, payloads, listenerOne.ReceivedPayloads())
	require.ElementsMatch(t, payloads, listenerTwo.ReceivedPayloads())
}
