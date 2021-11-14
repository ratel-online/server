package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"github.com/ratel-online/server/model"
)

type new struct{}

func (*new) Init(player *model.Player) error {
	return player.WriteString("New Room\n")
}

func (*new) Next(player *model.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	for id, name := range consts.GameTypes {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, name))
	}
	buf.WriteString("Type: ")
	gameType, err := player.AskForInt(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	if _, ok := consts.GameTypes[gameType]; !ok {
		return 0, player.WriteError(consts.ErrorsGameTypeInvalid)
	}
	players, err := player.AskForInt("Players: ")
	if err != nil {
		return 0, player.WriteError(err)
	}
	if players < consts.MinPlayers || players > consts.MaxPlayers {
		return 0, player.WriteError(consts.ErrorsPlayersInvalid)
	}
	robots, err := player.AskForInt("Robots: ")
	if err != nil {
		return 0, player.WriteError(err)
	}
	if robots < consts.MinPlayers || robots > consts.MaxPlayers {
		return 0, player.WriteError(consts.ErrorsRobotsInvalid)
	}
	room := database.CreateRoom(player.ID)
	room.Type = gameType
	room.Players = players
	room.Robots = robots
	err = database.JoinRoom(room.ID, player.ID)
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*new) Back(player *model.Player) consts.StateID {
	return consts.StateHome
}
