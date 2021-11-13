package state

import (
    "github.com/ratel-online/server/consts"
    "github.com/ratel-online/server/model"
)

type panelMode struct{}

func (*panelMode) Apply(player *model.Player) error {
    return player.WriteString(`Select Mode:
1. PvP
2. PvE`)
}

func (*panelMode) Next(player *model.Player) (consts.StateID, error) {
    packet, err := player.Read()
    if err != nil {
        return 0, err
    }
    selected := packet.Int()
    if selected == 1 {
        return consts.PanelPvp, nil
    } else if selected == 2 {
        return consts.PanelPve, nil
    }
    return 0, player.WriteError(consts.ErrorsInvalidInput)
}
