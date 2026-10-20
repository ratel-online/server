package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type ColoredCard struct {
	card  Card
	color color.Color
}

func NewColoredCard(card Card, color color.Color) ColoredCard {
	return ColoredCard{
		card:  card,
		color: color,
	}
}

func (c ColoredCard) Actions() []action.Action {
	return c.card.Actions()
}

func (c ColoredCard) Color() color.Color {
	return c.color
}

func (c ColoredCard) Equal(other Card) bool {
	return c.card.Equal(other)
}

func (c ColoredCard) String() string {
	return c.color.Paint(c.card.String())
}
