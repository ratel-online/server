package state

import (
    "github.com/ratel-online/server/consts"
    "github.com/ratel-online/server/model"
)

type welcome struct{}

func (*welcome) Apply(player *model.Player) error {
    return player.WriteString(`Welcome to ratel-online.`)
}

func (*welcome) Next(player *model.Player) (consts.StateID, error) {
    return consts.PanelMode, nil
}
