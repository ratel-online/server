package consts

import (
	"fmt"
	"github.com/ratel-online/core/consts"
	"github.com/ratel-online/server/config"
	"time"
)

type StateID int

const (
	_ StateID = iota
	StateWelcome
	StateHome
	StateJoin
	StateNew
	StateSetting
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
	MaxPlayers = config.ALLOW_ROOM_PLAYER_NUM

	RoomStateWaiting = 1
	RoomStateRunning = 2

	GameTypeClassic = 1
	GameTypeLaiZi   = 2
	GameTypeSkill   = 3

	RobTimeout  = 20 * time.Second
	PlayTimeout = 40 * time.Second
)

// Room properties.
const (
	RoomPropsDotShuffle = "ds"
	RoomPropsLaiZi      = "lz"
	RoomPropsSkill      = "sk"
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
	ErrorsVersion                = NewErr(1, true, "client version is too low.You must update your client. ")
	ErrorsRoomInvalid            = NewErr(1, true, "Room invalid. ")
	ErrorsGameTypeInvalid        = NewErr(1, false, "Game type invalid. ")
	ErrorsPasswordTooLong        = NewErr(1, false, "Your password is too long, must less 10 charts. ")
	ErrorsPlayerTooMany          = NewErr(1, false, fmt.Sprintf("Too many players.Must less than %d. ", config.ALLOW_ROOM_PLAYER_NUM))
	ErrorsPlayerTooLittle        = NewErr(1, false, "Too little players.Must greater than 1. ")
	ErrorsRoomPlayersIsFull      = NewErr(1, false, "Room players is fill. ")
	ErrorsRoomPassword           = NewErr(1, false, "Room password error. ")
	ErrorsJoinFailForRoomRunning = NewErr(1, false, "Join fail, room is running. ")
	ErrorsGamePlayersInvalid     = NewErr(1, false, "Game players invalid. ")
	ErrorsPokersFacesInvalid     = NewErr(1, false, "Pokers faces invalid. ")
	ErrorsHaveToPlay             = NewErr(1, false, "Have to play. ")

	GameTypes = map[int]string{
		GameTypeClassic: "Classic",
		GameTypeLaiZi:   "LaiZi",
		GameTypeSkill:   "Skill",
		//GameTypeRunFast: "RunFast",
	}
	GameTypesIds = []int{GameTypeClassic, GameTypeLaiZi, GameTypeSkill} // GameTypeLaiZi, GameTypeRunFast
	RoomStates   = map[int]string{
		RoomStateWaiting: "Waiting",
		RoomStateRunning: "Running",
	}
)
