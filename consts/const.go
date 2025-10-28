package consts

import (
	"time"

	"github.com/ratel-online/core/consts"
)

type StateID int

const (
	_ StateID = iota
	StateWelcome
	StateHome
	StateJoin
	StateCreate
	StateWaiting
	StateGame
	StateRunFastGame
	StateUnoGame
	StateMahjongGame
	StateTexasGame
)

type SkillID int

const (
	_ SkillID = iota - 1
	SkillWYSS
	SkillHYJJ
	SkillGHJM
	SkillPFCZ
	SkillDHXJ
	SkillLJFZ
	SkillZWZB
	SkillSKLF
	Skill996
	SkillTZJW
)

const (
	IsStart = consts.IsStart
	IsStop  = consts.IsStop

	MinPlayers = 3
	// MaxPlayers https://github.com/ratel-online/server/issues/14 小鄧修改
	MaxPlayers = 3

	RoomStateWaiting = 1
	RoomStateRunning = 2

	GameTypeClassic = 1
	GameTypeLaiZi   = 2
	GameTypeSkill   = 3
	GameTypeRunFast = 4
	GameTypeTexas   = 5
	GameTypeUno     = 6
	GameTypeMahjong = 7

	RobTimeout         = 20 * time.Second
	PlayTimeout        = 40 * time.Second
	PlayMahjongTimeout = 30 * time.Second
	BetTimeout         = 60 * time.Second
)

// Room properties.
const (
	RoomPropsDotShuffle = "ds"
	RoomPropsLaiZi      = "lz"
	RoomPropsSkill      = "sk"
	RoomPropsPassword   = "pwd"
	RoomPropsPlayerNum  = "pn"
	RoomPropsChat       = "ct"
)

var MnemonicSorted = []int{15, 14, 2, 1, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3}

var RunFastMnemonicSorted = []int{2, 1, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3}

type Error struct {
	Code int
	Msg  string
	Exit bool
}

func (e Error) Error() string {
	return e.Msg
}

func NewErr(code int, exit bool, msg string) Error {
	return Error{Code: code, Exit: exit, Msg: msg}
}

var (
	ErrorsExist                   = NewErr(1, true, "Exist. ")
	ErrorsChanClosed              = NewErr(1, true, "Chan closed. ")
	ErrorsTimeout                 = NewErr(1, false, "Timeout. ")
	ErrorsInputInvalid            = NewErr(1, false, "Input invalid. ")
	ErrorsChatUnopened            = NewErr(1, false, "Chat disabled. ")
	ErrorsAuthFail                = NewErr(1, true, "Auth fail. ")
	ErrorsRoomInvalid             = NewErr(1, true, "Room invalid. ")
	ErrorsGameTypeInvalid         = NewErr(1, false, "Game type invalid. ")
	ErrorsRoomPlayersIsFull       = NewErr(1, false, "Room players is fill. ")
	ErrorsRoomPassword            = NewErr(1, false, "Sorry! Password incorrect! ")
	ErrorsJoinFailForRoomRunning  = NewErr(1, false, "Join fail, room is running. ")
	ErrorsJoinFailForKicked       = NewErr(1, false, "Join fail, you have been kicked from this room. ")
	ErrorsGamePlayersInvalid      = NewErr(1, false, "Game players invalid. ")
	ErrorsPokersFacesInvalid      = NewErr(1, false, "Pokers faces invalid. ")
	ErrorsHaveToPlay              = NewErr(1, false, "Have to play. ")
	ErrorsMustHaveToPlay          = NewErr(1, false, "There is a hand that can be played and must be played. ")
	ErrorsEndToPlay               = NewErr(1, false, "Can only come out at the end. ")
	ErrorsUnknownTexasRound       = NewErr(1, false, "Unknown texas round. ")
	ErrorsGamePlayersInsufficient = NewErr(1, false, "Game players insufficient. ")
	ErrorsCannotKickYourself      = NewErr(1, false, "Cannot kick yourself. ")
	ErrorsPlayerNotInRoom         = NewErr(1, true, "Player not in room. ")

	GameTypes = map[int]string{
		GameTypeClassic: "斗地主",
		GameTypeLaiZi:   "斗地主-癞子版",
		GameTypeSkill:   "斗地主-大招版",
		GameTypeRunFast: "跑得快",
		//GameTypeUno:     "Uno",
		//GameTypeMahjong: "Mahjong",
		GameTypeTexas: "德州扑克",
	}
	GameTypesIds = []int{GameTypeClassic, GameTypeLaiZi, GameTypeSkill, GameTypeRunFast, GameTypeTexas}
	RoomStates   = map[int]string{
		RoomStateWaiting: "Waiting",
		RoomStateRunning: "Running",
	}
)
