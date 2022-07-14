package player

import (
	"math/rand"

	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
	"github.com/ratel-online/server/uno/game"
)

type naivePlayer struct {
	basicPlayer
}

func NewNaivePlayer(name string) game.Player {
	return naivePlayer{basicPlayer: basicPlayer{name: name}}
}

func (p naivePlayer) PickColor(gameState game.State) color.Color {
	randomIndex := rand.Intn(4)
	randomColor := allColors[randomIndex]
	return randomColor
}

func (p naivePlayer) Play(playableCards []card.Card, gameState game.State) card.Card {
	firstCard := playableCards[0]
	return firstCard
}

var allColors = []color.Color{
	color.Red,
	color.Yellow,
	color.Blue,
	color.Green,
}
