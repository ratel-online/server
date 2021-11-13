package consts

import "errors"

type StateID int

const (
    _ StateID = iota
    Welcome
    PanelMode
    PanelPvp
    PanelPve
)

var (
    ErrorsInvalidInput = errors.New("Invalid input. ")
    ErrorsAuthFail     = errors.New("Auth fail. ")
)
