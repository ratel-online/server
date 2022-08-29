package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type create struct{}

func (*create) Next(player *database.Player) (consts.StateID, error) {
	gameType, err := askForGameType(player)
	if err != nil {
		return 0, err
	}
	// 创建房间
	room := database.CreateRoom(player.ID, gameType)
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

func (*create) Exit(_ *database.Player) consts.StateID {
	return consts.StateHome
}

// 询问游戏类型
func askForGameType(player *database.Player) (gameType int, err error) {
	buf := bytes.Buffer{}
	buf.WriteString("Please select game type\n")
	for _, id := range consts.GameTypesIds {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, consts.GameTypes[id]))
	}
	err = player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	gameType, err = player.AskForInt()
	if err != nil {
		_ = player.WriteError(consts.ErrorsGameTypeInvalid)
		return 0, consts.ErrorsGameTypeInvalid
	}
	// 游戏类型输入非法
	if _, ok := consts.GameTypes[gameType]; !ok {
		_ = player.WriteError(consts.ErrorsGameTypeInvalid)
		return 0, consts.ErrorsGameTypeInvalid
	}
	return
}
