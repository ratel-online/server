package state

import (
	"github.com/ratel-online/core/log"
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
	Next(player *model.Player) (consts.StateID, error)
}

func Root() consts.StateID {
	return consts.Welcome
}

func Load(player *model.Player) error{
	var err error
	for{
		state := states[player.GetState()]
		err = state.Apply(player)
		if err != nil{
			log.Error(err)
			break
		}
		stateId, err := state.Next(player)
		if err != nil{
			log.Error(err)
			break
		}
		if stateId > 0{
			player.State(stateId)
		}
	}
	return err
}