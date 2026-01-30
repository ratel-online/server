package game

import (
    "github.com/ratel-online/server/consts"
    "github.com/ratel-online/server/database"
)

type Liar struct{}

func (g *Liar) Next(player *database.Player) (consts.StateID, error) {
    // 这里编写游戏的主要循环逻辑
    // 例如：等待发牌、处理玩家出牌输入、判断胜负等
    room := database.GetRoom(player.RoomID)
    if room == nil {
    	return 0, player.WriteError(consts.ErrorsExist)
    }
    game := room.Game.(*database.Mahjong)
    buf := bytes.Buffer{}

    buf
    return 0, nil
}

func (g *Liar) Exit(player *database.Player) consts.StateID {
    return consts.StateHome
}