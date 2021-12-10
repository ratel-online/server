package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state/classics"
	"github.com/ratel-online/server/state/laizi"
	"runtime"
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
	Next(player *database.Player) (consts.StateID, error)
	Exit(player *database.Player) consts.StateID
}

func Run(player *database.Player) {
	player.State(consts.StateWelcome)
	defer func() {
		if err := recover(); err != nil {
			buf := bytes.Buffer{}
			buf.WriteString(fmt.Sprintf("%v\n", err))
			for i := 1; ; i++ {
				pc, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				buf.WriteString(fmt.Sprintf("%s:%d (0x%x)\n", file, line, pc))
			}
			fmt.Println(buf.String())
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
				state.Exit(player)
				log.Error(err)
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
