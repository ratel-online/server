package event_test

import (
	"testing"

	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/event"
	"github.com/stretchr/testify/require"
)

func TestCardPlayed(t *testing.T) {
	listenerOne := event.NewDummyListener()
	listenerTwo := event.NewDummyListener()

	event.CardPlayed.AddListener(listenerOne)
	event.CardPlayed.AddListener(listenerTwo)

	payloads := []event.CardPlayedPayload{
		{
			PlayerName: "Someone",
			Card:       card.NewWildCard(),
		},
		{
			PlayerName: "Somebody",
			Card:       card.NewDrawTwoCard(color.Green),
		},
	}

	for _, payload := range payloads {
		event.CardPlayed.Emit(payload)
	}

	require.ElementsMatch(t, payloads, listenerOne.ReceivedPayloads())
	require.ElementsMatch(t, payloads, listenerTwo.ReceivedPayloads())
}
