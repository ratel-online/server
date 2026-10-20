package card

import (
	"github.com/feel-easy/uno/card/action"
	"github.com/feel-easy/uno/card/color"
)

type Card interface {
	Actions() []action.Action
	Color() color.Color
	Equal(other Card) bool
	String() string
}
