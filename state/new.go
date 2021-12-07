package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
)

type new struct{}

func (*new) Next(player *model.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	buf.WriteString("Please select game type\n")
	for _, id := range consts.GameTypesIds {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, consts.GameTypes[id]))
	}
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	gameType, err := player.AskForInt()
	if err != nil {
		return 0, player.WriteError(err)
	}
	if _, ok := consts.GameTypes[gameType]; !ok {
		return 0, player.WriteError(consts.ErrorsGameTypeInvalid)
	}
	room := database.CreateRoom(player.ID)
	room.Type = gameType
	err = player.WriteString(fmt.Sprintf("Create room successful, id : %d\n", room.ID))
	if err != nil {
		return 0, player.WriteError(err)
	}
	err = database.JoinRoom(room.ID, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*new) Exit(player *model.Player) consts.StateID {
	return consts.StateHome
}
