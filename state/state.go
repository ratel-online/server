package state

import (
	"strings"
	"time"

	"github.com/ratel-online/server/state/game/texas"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/state/game"
)

var states = map[consts.StateID]State{}

func init() {
	register(consts.StateWelcome, &welcome{})
	register(consts.StateHome, &home{})
	register(consts.StateJoin, &join{})
	register(consts.StateCreate, &create{})
	register(consts.StateWaiting, &waiting{})
	register(consts.StateGame, &game.Game{})
	register(consts.StateUnoGame, &game.Uno{})
	register(consts.StateRunFastGame, &game.RunFastGame{})
	register(consts.StateMahjongGame, &game.Mahjong{})
	register(consts.StateTexasGame, &texas.Texas{})
	register(consts.StateLiarGame, &game.Liar{})
	register(consts.StateUndercoverGame, &game.Undercover{})
}

func register(id consts.StateID, state State) {
	states[id] = state
}

type State interface {
	Next(player *database.Player) (consts.StateID, error)
	Exit(player *database.Player) consts.StateID
}

func Run(player *database.Player) {
	RunFrom(player, consts.StateWelcome)
}

// RunFrom 从指定状态启动状态机，顶号时用于回到玩家原本所在的状态。
func RunFrom(player *database.Player, initial consts.StateID) {
	gen := player.BeginStateRun()
	player.State(initial)
	defer func() {
		if err := recover(); err != nil {
			async.PrintStackTrace(err)
		}
		log.Infof("player %s state machine break up.\n", player)
	}()
	for {
		// 被顶号后，上一代状态机必须退出，否则两个协程会抢同一个输入通道
		if !player.IsCurrentRun(gen) {
			log.Infof("player %s state machine superseded by a newer session, exit.\n", player)
			return
		}
		state := states[player.GetState()]
		stateId, err := state.Next(player)
		if err != nil {
			if !player.IsCurrentRun(gen) {
				return
			}
			// 牌局进行中掉线：座位保留，重连窗口内等本人回来，
			// 超时后由托管继续把这局打完（输入侧见 database.Player.botInput）。
			if player.CanBotContinue(gen) {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			// 断线玩家不再托管（牌局结束/座位已不在）：顺手把残留在房间里的座位清掉，
			// 避免房间一直挂着一个永远不在线的空位。
			if roomID := player.RoomIDOf(); !player.IsOnline() && roomID != 0 {
				database.LeaveRoom(roomID, player.ID)
			}
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
		if !player.IsCurrentRun(gen) {
			return
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
