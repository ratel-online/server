package consts

import (
	"fmt"
	"github.com/ratel-online/core/consts"
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
	StateClassics
)

const (
	IS = consts.IS

	MinPlayers = 3
	MaxPlayers = 6

	RoomStateWaiting = 1
	RoomStateRunning = 2

	GameTypeClassic = 1
	GameTypeLaiZi   = 2
	GameTypeRunFast = 3

	ClassicsRobTimeout  = 1000 * time.Second
	ClassicsPlayTimeout = 30 * time.Second
	ClassicsLostTimeout = 1 * time.Second
)

type Error struct {
	Msg  string
	Exit bool
}

func (e Error) Error() string {
	return e.Msg
}

func NewErr(msg string) Error {
	return Error{Msg: msg, Exit: false}
}

func NewExitErr(msg string) Error {
	return Error{Msg: msg, Exit: true}
}

var (
	ErrorsChanClosed             = NewExitErr("Chan closed. ")
	ErrorsTimeout                = NewErr("Timeout. ")
	ErrorsExist                  = NewExitErr("Exist. ")
	ErrorsInputInvalid           = NewErr("Input invalid. ")
	ErrorsAuthFail               = NewExitErr("Auth fail. ")
	ErrorsRoomInvalid            = NewExitErr("Room invalid. ")
	ErrorsPlayersInvalid         = NewExitErr(fmt.Sprintf("Invalid players, must %d-%d", MinPlayers, MaxPlayers))
	ErrorsGameTypeInvalid        = NewErr("Game type invalid. ")
	ErrorsRoomPlayersIsFull      = NewErr("Room players is fill. ")
	ErrorsJoinFailForRoomRunning = NewErr("Join fail, room is running. ")
	ErrorsGamePlayersInvalid     = NewErr("Game players invalid. ")
	ErrorsPokersFacesInvalid     = NewErr("Pokers faces invalid. ")

	GameTypes = map[int]string{
		GameTypeClassic: "Classic",
		GameTypeLaiZi:   "LaiZi",
		GameTypeRunFast: "RunFast",
	}
	GameTypesIds = []int{GameTypeClassic} // GameTypeLaiZi, GameTypeRunFast
	RoomStates   = map[int]string{
		RoomStateWaiting: "Waiting",
		RoomStateRunning: "Running",
	}
)
