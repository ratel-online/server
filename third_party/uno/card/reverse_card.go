package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type ReverseCard struct {
	color color.Color
}

func NewReverseCard(color color.Color) ReverseCard {
	return ReverseCard{color: color}
}

func (c ReverseCard) Actions() []action.Action {
	return []action.Action{
		action.NewReverseTurnsAction(),
	}
}

func (c ReverseCard) Color() color.Color {
	return c.color
}

func (c ReverseCard) Equal(other Card) bool {
	_, typeMatched := other.(ReverseCard)
	return typeMatched && c.color == other.Color()
}

func (c ReverseCard) String() string {
	return c.color.Paint("<=>")
}
