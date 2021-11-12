package state

import (
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/model"
)

type panelMode struct{}

func (*panelMode) Apply(player *model.Player) error {
	return player.WriteString(`
                Select Mode:
                1. PvP
                2. PvE
            `)
}

func (*panelMode) Next(player *model.Player, packet protocol.Packet) (consts.StateID, error) {
	selected := packet.Int()
	if selected == 1 {
		return consts.PanelPvp, nil
	} else if selected == 2 {
		return consts.PanelPve, nil
	}
	return 0, consts.ErrorsInvalidInput
}
