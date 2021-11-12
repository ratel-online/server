package state

import (
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
)

var states = map[consts.StateID]State{}

func init() {
	register(consts.Welcome, &welcome{})
	register(consts.PanelMode, &panelMode{})
}

func register(id consts.StateID, state State) {
	states[id] = state
}

type State interface {
	Apply(player *model.Player) error
	Next(player *model.Player, packet protocol.Packet) (consts.StateID, error)
}

func Root() consts.StateID {
	return consts.Welcome
}

func Play(play *model.Player) {
	s := states[play.GetState()]
}
