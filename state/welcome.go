package state

import (
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
)

type welcome struct{}

func (*welcome) Next(player *model.Player) (consts.StateID, error) {
	err := player.WriteString("Welcome to ratel online! \n")
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateHome, nil
}

func (*welcome) Exit(player *model.Player) consts.StateID {
	return 0
}
