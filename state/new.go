package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/config"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type new struct{}

func (*new) Next(player *database.Player) (consts.StateID, error) {

	gameType, err := askGameType(player)
	if err != nil {
		return 0, err
	}

	password, err := askPassword(player)
	if err != nil {
		return 0, err
	}

	playerNum, err := askGamePlayerNum(player)
	if err != nil {
		return 0, err
	}

	// 创建房间资源
	room := database.CreateRoom(player.ID, password, playerNum)
	room.Type = gameType
	err = player.WriteString(fmt.Sprintf("Create room successful, id : %d\n", room.ID))
	if err != nil {
		return 0, player.WriteError(err)
	}
	err = database.JoinRoom(room.ID, player.ID, password)
	if err != nil {
		return 0, player.WriteError(err)
	}
	return consts.StateWaiting, nil
}

func (*new) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}

// 询问游戏玩家数量
func askGamePlayerNum(player *database.Player) (num int, err error) {

	err = player.WriteString(fmt.Sprintf("Please set the number of players  , default is %d \n", config.ALLOW_ROOM_PLAYER_NUM))
	if err != nil {
		return 3, player.WriteError(err)
	}
	num, err = player.AskForInt() // 等待用户输入

	if num == 0 || err != nil {
		num = config.ALLOW_ROOM_PLAYER_NUM
	}

	// 房间最大人数限制
	if num > config.ALLOW_ROOM_PLAYER_NUM {
		_ = player.WriteString(fmt.Sprintf("Too many players.Must less than %d. \n", config.ALLOW_ROOM_PLAYER_NUM))
		return 0, consts.ErrorsPlayerTooMany
	}

	if num <= 1 {
		_ = player.WriteString(fmt.Sprintf("Too many players.Must greater than 1. \n"))
		return 0, consts.ErrorsPlayerTooLittle
	}

	if err != nil {
		return config.ALLOW_ROOM_PLAYER_NUM, nil
	}

	return
}

// 询问房间密码设置
func askPassword(player *database.Player) (password string, err error) {
	buf := bytes.Buffer{}
	buf.WriteString("Please set room password , default is null \n")
	err = player.WriteString(buf.String())
	if err != nil {
		return "", player.WriteError(err)
	}
	password, err = player.AskForString() // 等待用户输入

	if err != nil {
		log.Errorf("user input error! %+v", err)
		password = ""
	}

	// 不允许10位以上的密码，防止恶意输入超长文本占满服务器资源
	if len(password) > 10 {
		return "", consts.ErrorsPasswordTooLong
	}

	if err != nil {
		return "", player.WriteError(err)
	}

	return
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
