package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type DrawTwoCard struct {
	color color.Color
}

func NewDrawTwoCard(color color.Color) DrawTwoCard {
	return DrawTwoCard{color: color}
}

func (c DrawTwoCard) Actions() []action.Action {
	return []action.Action{
		action.NewSkipTurnAction(),
		action.NewDrawCardsAction(2),
	}
}

func (c DrawTwoCard) Color() color.Color {
	return c.color
}

func (c DrawTwoCard) Equal(other Card) bool {
	_, typeMatched := other.(DrawTwoCard)
	return typeMatched && c.color == other.Color()
}

func (c DrawTwoCard) String() string {
	return c.color.Paint("+2!")
}
