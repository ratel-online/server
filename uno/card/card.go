package card

import (
	"github.com/ratel-online/server/uno/card/action"
	"github.com/ratel-online/server/uno/card/color"
)

type Card interface {
	Actions() []action.Action
	Color() color.Color
	Equal(other Card) bool
	String() string
}
