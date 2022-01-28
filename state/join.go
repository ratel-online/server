package state

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"strconv"
)

type join struct{}

func (s *join) Next(player *database.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	rooms := database.GetRooms()
	buf.WriteString(fmt.Sprintf("%-10s%-10s%-10s%-10s\n", "ID", "Type", "Players", "State"))
	for _, room := range rooms {

		// 游戏进行中或者已经满了就不展示在房间列表里面了
		if room.Players >= room.MaxPlayer || room.State == consts.RoomStateRunning {
			continue
		}

		// 密码房id前面加星号
		if room.Password != "" {
			buf.WriteString(fmt.Sprintf("*%-10d%-10s%-10d%-10s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
		} else {
			buf.WriteString(fmt.Sprintf("%-10d%-10s%-10d%-10s\n", room.ID, consts.GameTypes[room.Type], room.Players, consts.RoomStates[room.State]))
		}
	}
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	signal, err := player.AskForString()
	if err != nil {
		return 0, player.WriteError(err)
	}
	if isExit(signal) {
		return s.Exit(player), nil
	}
	if isLs(signal) {
		return consts.StateJoin, nil
	}
	roomId, err := strconv.ParseInt(signal, 10, 64)
	if err != nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}
	room := database.GetRoom(roomId)
	if room == nil {
		return 0, player.WriteError(consts.ErrorsRoomInvalid)
	}

	//房间存在密码，要求输入密码
	if room.Password != "" {
		buf = bytes.Buffer{}
		buf.WriteString("Please input room password. \n")
		err = player.WriteString(buf.String())

		if err != nil {
			return 0, player.WriteError(err)
		}

		password, err := player.AskForString()
		if err != nil {
			return 0, player.WriteError(err)
		}

		if password != room.Password {
			buf = bytes.Buffer{}
			buf.WriteString("sorry! password incorrect. \n")
			err = player.WriteString(buf.String())
			if err != nil {
				return 0, player.WriteError(err)
			}

			return 0, consts.ErrorsRoomPassword
		}

	}

	err = database.JoinRoom(roomId, player.ID, room.Password)
	if err != nil {
		return 0, player.WriteError(err)
	}
	database.Broadcast(roomId, fmt.Sprintf("%s joined room! room current has %d players\n", player.Name, room.Players))
	return consts.StateWaiting, nil
}

func (*join) Exit(player *database.Player) consts.StateID {
	return consts.StateHome
}
