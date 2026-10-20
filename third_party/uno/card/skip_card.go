package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type SkipCard struct {
	color color.Color
}

func NewSkipCard(color color.Color) SkipCard {
	return SkipCard{color: color}
}

func (c SkipCard) Actions() []action.Action {
	return []action.Action{
		action.NewSkipTurnAction(),
	}
}

func (c SkipCard) Color() color.Color {
	return c.color
}

func (c SkipCard) Equal(other Card) bool {
	_, typeMatched := other.(SkipCard)
	return typeMatched && c.color == other.Color()
}

func (c SkipCard) String() string {
	return c.color.Paint("(/)")
}
