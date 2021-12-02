package consts

import (
	"errors"
	"fmt"
	"github.com/ratel-online/core/consts"
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

	MinPlayers       = 3
	MaxPlayers       = 6
	RoomStateWaiting = 1
	RoomStateRunning = 2
	GameTypeClassic  = 1
	GameTypeLaiZi    = 2
	GameTypeRunFast  = 3
)

var (
	ErrorsExist                  = errors.New("Exist. ")
	ErrorsInputInvalid           = errors.New("Input invalid. ")
	ErrorsAuthFail               = errors.New("Auth fail. ")
	ErrorsRoomInvalid            = errors.New("Room invalid. ")
	ErrorsPlayersInvalid         = errors.New(fmt.Sprintf("Invalid players, must %d-%d", MinPlayers, MaxPlayers))
	ErrorsRobotsInvalid          = errors.New(fmt.Sprintf("Invalid robots, must %d-%d", MinPlayers, MaxPlayers))
	ErrorsGameTypeInvalid        = errors.New("Game type invalid. ")
	ErrorsRoomPlayersIsFull      = errors.New("Room players is fill. ")
	ErrorsJoinFailForRoomRunning = errors.New("Join fail, room is running. ")

	GameTypes = map[int]string{
		GameTypeClassic: "Classic",
		GameTypeLaiZi:   "LaiZi",
		GameTypeRunFast: "RunFast",
	}
	GameTypesIds = []int{GameTypeClassic, GameTypeLaiZi, GameTypeRunFast}
	RoomStates   = map[int]string{
		RoomStateWaiting: "Waiting",
		RoomStateRunning: "Running",
	}
)
