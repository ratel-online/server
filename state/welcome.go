package state

import (
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/service"
)

type welcome struct{}

func (*welcome) Next(player *service.Player) (consts.StateID, error) {
	err := player.WriteString(fmt.Sprintf("Hi %s, Welcome to ratel online! \n", player.Name))
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateHome, nil
}

func (*welcome) Exit(player *service.Player) consts.StateID {
	return 0
}
