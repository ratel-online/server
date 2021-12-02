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
	buf.WriteString("Game type: \n")
	for _, id := range consts.GameTypesIds {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, consts.GameTypes[id]))
	}
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	gameType, err := player.AskForInt(player.Terminal())
	if err != nil {
		return 0, player.WriteError(err)
	}
	if _, ok := consts.GameTypes[gameType]; !ok {
		return 0, player.WriteError(consts.ErrorsGameTypeInvalid)
	}

	err = player.WriteString("Player number: \n")
	if err != nil {
		return 0, player.WriteError(err)
	}

	players, err := player.AskForInt(player.Terminal())
	if err != nil {
		return 0, player.WriteError(err)
	}
	if players < consts.MinPlayers || players > consts.MaxPlayers {
		return 0, player.WriteError(consts.ErrorsPlayersInvalid)
	}
	room := database.CreateRoom(player.ID)
	room.Type = gameType
	room.Players = players
	err = database.JoinRoom(room.ID, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*new) Back(player *model.Player) consts.StateID {
	return consts.StateHome
}
