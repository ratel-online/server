package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type new struct{}

func (*new) Next(player *database.Player) (consts.StateID, error) {

	gameType, err := askGameType(player)
	if err != nil {
		return 0, err
	}

	// 创建房间资源
	room := database.CreateRoom(player.ID, "", consts.MaxPlayers)
	room.Type = gameType
	err = player.WriteString(fmt.Sprintf("Create room successful, id : %d\n", room.ID))
	if err != nil {
		return 0, player.WriteError(err)
	}
	err = database.JoinRoom(room.ID, player.ID, "")
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*new) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

// 询问游戏类型
func askGameType(player *database.Player) (gameType int, err error) {
	buf := bytes.Buffer{}
	buf.WriteString("Please select game type\n")
	for _, id := range consts.GameTypesIds {
		buf.WriteString(fmt.Sprintf("%d.%s\n", id, consts.GameTypes[id]))
	}
	err = player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	gameType, err = player.AskForInt() // 等待用户输入
	if err != nil {
		return 0, player.WriteError(err)
	}

	// 游戏类型输入非法
	if _, ok := consts.GameTypes[gameType]; !ok {
		return 0, player.WriteError(consts.ErrorsGameTypeInvalid)
	}
	return
}
