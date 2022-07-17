package event_test

import (
	"testing"

	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/event"
	"github.com/stretchr/testify/require"
)

func TestFirstCardPlayed(t *testing.T) {
	listenerOne := event.NewDummyListener()
	listenerTwo := event.NewDummyListener()

	event.FirstCardPlayed.AddListener(listenerOne)
	event.FirstCardPlayed.AddListener(listenerTwo)

	payloads := []event.FirstCardPlayedPayload{
		{
			Card: card.NewWildCard(),
		},
		{
			Card: card.NewDrawTwoCard(color.Green),
		},
	}

	for _, payload := range payloads {
		event.FirstCardPlayed.Emit(payload)
	}

	require.ElementsMatch(t, payloads, listenerOne.ReceivedPayloads())
	require.ElementsMatch(t, payloads, listenerTwo.ReceivedPayloads())
}
