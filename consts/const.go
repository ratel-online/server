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
	MaxPlayers = 6

	RoomStateWaiting = 1
	RoomStateRunning = 2

	GameTypeClassic = 1
	GameTypeLaiZi   = 2
	GameTypeSkill   = 3
	GameTypeUno     = 4

	RobTimeout  = 20 * time.Second
	PlayTimeout = 40 * time.Second
)

// Room properties.
const (
	RoomPropsDotShuffle = "ds"
	RoomPropsLaiZi      = "lz"
	RoomPropsSkill      = "sk"
	RoomPropsPassword   = "pwd"
	RoomPropsPlayerNum  = "pn"
)

var MnemonicSorted = []int{15, 14, 2, 1, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3}

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
	ErrorsExist                  = NewErr(1, true, "Exist. ")
	ErrorsChanClosed             = NewErr(1, true, "Chan closed. ")
	ErrorsTimeout                = NewErr(1, false, "Timeout. ")
	ErrorsInputInvalid           = NewErr(1, false, "Input invalid. ")
	ErrorsAuthFail               = NewErr(1, true, "Auth fail. ")
	ErrorsRoomInvalid            = NewErr(1, true, "Room invalid. ")
	ErrorsGameTypeInvalid        = NewErr(1, false, "Game type invalid. ")
	ErrorsRoomPlayersIsFull      = NewErr(1, false, "Room players is fill. ")
	ErrorsRoomPassword           = NewErr(1, false, "Sorry! Password incorrect! ")
	ErrorsJoinFailForRoomRunning = NewErr(1, false, "Join fail, room is running. ")
	ErrorsGamePlayersInvalid     = NewErr(1, false, "Game players invalid. ")
	ErrorsPokersFacesInvalid     = NewErr(1, false, "Pokers faces invalid. ")
	ErrorsHaveToPlay             = NewErr(1, false, "Have to play. ")

	GameTypes = map[int]string{
		GameTypeClassic: "Classic",
		GameTypeLaiZi:   "LaiZi",
		GameTypeSkill:   "Skill",
		GameTypeUno:     "uno",
	}
	GameTypesIds = []int{GameTypeClassic, GameTypeLaiZi, GameTypeSkill, GameTypeUno} // GameTypeLaiZi, GameTypeRunFast
	RoomStates   = map[int]string{
		RoomStateWaiting: "Waiting",
		RoomStateRunning: "Running",
	}
)
