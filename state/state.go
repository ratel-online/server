package state

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
)

var states = map[consts.StateID]State{}

func init() {
	register(consts.StateWelcome, &welcome{})
	register(consts.StateHome, &home{})
	register(consts.StateJoin, &join{})
	register(consts.StateNew, &new{})
}

func register(id consts.StateID, state State) {
	states[id] = state
}

type State interface {
	Init(player *model.Player) error
	Next(player *model.Player) (consts.StateID, error)
	Back(player *model.Player) consts.StateID
}

func Root() consts.StateID {
	return consts.StateWelcome
}

func Load(player *model.Player) error {
	var err error
	for {
		state := states[player.GetState()]
		err = state.Init(player)
		if err != nil {
			log.Error(err)
			break
		}
		stateId, err := state.Next(player)
		if err != nil {
			if err == consts.ErrorsExist {
				stateId = state.Back(player)
			} else {
				log.Error(err)
				break
			}
		}
		if stateId > 0 {
			player.State(stateId)
		}
	}
	return err
}
