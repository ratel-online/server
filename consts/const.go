package consts

import "errors"

type StateID int

const (
	Welcome StateID = 1
	PanelMode
	PanelPvp
	PanelPve
)

var (
	ErrorsInvalidInput = errors.New("Invalid input. ")
	ErrorsAuthFail     = errors.New("Auth fail. ")
)
