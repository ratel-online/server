package database

import (
	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/server/model"
)

var players = map[int64]*model.Player{}

func RegisterPlayer(conn *network.Conn, info *modelx.AuthInfo) *model.Player {
	player := &model.Player{
		ID:    info.ID,
		Name:  info.Name,
		Score: info.Score,
	}
	player.Conn(conn)
	players[conn.ID()] = player
	return player
}

func CancelPlayer(conn *network.Conn) {
	delete(players, conn.ID())
}
