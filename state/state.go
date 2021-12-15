package state

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/service"
	"github.com/ratel-online/server/state/classics"
	"github.com/ratel-online/server/state/laizi"
	"strings"
)

var states = map[consts.StateID]State{}

func init() {
	register(consts.StateWelcome, &welcome{})
	register(consts.StateHome, &home{})
	register(consts.StateJoin, &join{})
	register(consts.StateNew, &new{})
	register(consts.StateWaiting, &waiting{})
	register(consts.StateClassics, &classics.Classics{})
	register(consts.StateLaiZi, &laizi.LaiZi{})
}

func register(id consts.StateID, state State) {
	states[id] = state
}

type State interface {
	Next(player *service.Player) (consts.StateID, error)
	Exit(player *service.Player) consts.StateID
}

func Run(player *service.Player) {
	player.State(consts.StateWelcome)
	defer func() {
		if err := recover(); err != nil {
			async.PrintStackTrace(err)
		}
		log.Infof("player %s state machine break up.\n", player)
	}()
	for {
		state := states[player.GetState()]
		stateId, err := state.Next(player)
		if err != nil {
			if err1, ok := err.(consts.Error); ok {
				if err1.Exit {
					stateId = state.Exit(player)
				}
			} else {
				log.Error(err)
				state.Exit(player)
				break
			}
		}
		if stateId > 0 {
			player.State(stateId)
		}
	}
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
