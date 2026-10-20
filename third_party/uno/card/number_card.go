package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type NumberCard struct {
	color  color.Color
	number int
}

func NewNumberCard(color color.Color, number int) NumberCard {
	return NumberCard{
		color:  color,
		number: number,
	}
}

func (c NumberCard) Actions() []action.Action {
	return []action.Action{}
}

func (c NumberCard) Color() color.Color {
	return c.color
}

func (c NumberCard) Equal(other Card) bool {
	otherNumberCard, typeMatched := other.(NumberCard)
	return typeMatched && c.color == other.Color() && c.number == otherNumberCard.number
}

func (c NumberCard) Number() int {
	return c.number
}

func (c NumberCard) String() string {
	return c.color.Paintf("%d", c.number)
}
