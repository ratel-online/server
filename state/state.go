package state

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
	"strings"
)

var states = map[consts.StateID]State{}

func init() {
	register(consts.StateWelcome, &welcome{})
	register(consts.StateHome, &home{})
	register(consts.StateJoin, &join{})
	register(consts.StateNew, &new{})
	register(consts.StateWaiting, &waiting{})
	register(consts.StateClassics, &classics{})
}

func register(id consts.StateID, state State) {
	states[id] = state
}

type State interface {
	Next(player *model.Player) (consts.StateID, error)
	Exit(player *model.Player) consts.StateID
}

func Root() consts.StateID {
	return consts.StateWelcome
}

func Load(player *model.Player) error {
	var err error
	for {
		state := states[player.GetState()]
		stateId, err := state.Next(player)

		if err != nil {
			if err1, ok := err.(consts.Error); ok {
				if err1 == consts.ErrorsExist {
					stateId = state.Exit(player)
				}
			} else {
				state.Exit(player)
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

func isExit(signal string) bool {
	signal = strings.ToLower(signal)
	return isX(signal, "exit", "e")
}

func isLs(signal string) bool {
	return isX(signal, "ls")
}

func isX(signal string, x ...string) bool {
	signal = strings.ToLower(signal)
	for _, v := range x {
		if v == signal {
			return true
		}
	}
	return false
}
