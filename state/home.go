package state

import (
	"bytes"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
)

type home struct{}

func (*home) Init(player *model.Player) error {
	buf := bytes.Buffer{}
	buf.WriteString("1.Join\n")
	buf.WriteString("2.New\n")
	return player.WriteString(buf.String())
}

func (*home) Next(player *model.Player) (consts.StateID, error) {
	selected, err := player.AskForInt()
	if err != nil {
		return 0, player.WriteError(err)
	}
	if selected == 1 {
		return consts.StateJoin, nil
	} else if selected == 2 {
		return consts.StateNew, nil
	} else if selected == 3 {
		return consts.StateSetting, nil
	}
	return 0, player.WriteError(consts.ErrorsInputInvalid)
}

func (*home) Back(player *model.Player) consts.StateID {
	return 0
}
