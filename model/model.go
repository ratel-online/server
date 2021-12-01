package model

import (
	"fmt"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"strings"
)

type Player struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Score  int64  `json:"score"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	RoomID int64  `json:"roomId"`
	GameID int64  `json:"gameId"`

	conn  *network.Conn
	state consts.StateID
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) read() (*protocol.Packet, error) {
	return p.conn.Read()
}

func (p *Player) WriteString(data string) error {
	return p.conn.Write(protocol.Packet{
		Body: []byte(data),
	})
}

func (p *Player) WriteError(err error) error {
	if err == consts.ErrorsExist {
		return err
	}
	return p.conn.Write(protocol.Packet{
		Body: []byte(err.Error() + "\n"),
	})
}

func (p *Player) AskForPacket(msg ...string) (*protocol.Packet, error) {
	if len(msg) > 0 {
		err := p.WriteString(msg[0])
		if err != nil {
			return nil, err
		}
	}
	err := p.WriteString(consts.IS)
	if err != nil {
		return nil, err
	}
	packet, err := p.read()
	if err != nil {
		return nil, err
	}
	str := strings.ToLower(packet.String())
	if str == "exit" || str == "e" {
		return nil, consts.ErrorsExist
	}
	return packet, nil
}

func (p *Player) AskForInt(msg ...string) (int, error) {
	packet, err := p.AskForPacket(msg...)
	if err != nil {
		return 0, err
	}
	return packet.Int()
}

func (p *Player) AskForInt64(msg ...string) (int64, error) {
	packet, err := p.AskForPacket(msg...)
	if err != nil {
		return 0, err
	}
	return packet.Int64()
}

func (p *Player) AskForString(msg ...string) (string, error) {
	packet, err := p.AskForPacket(msg...)
	if err != nil {
		return "", err
	}
	return packet.String(), nil
}

func (p *Player) State(s consts.StateID) {
	p.state = s
}

func (p *Player) GetState() consts.StateID {
	return p.state
}

func (p *Player) Conn(conn *network.Conn) {
	p.conn = conn
}

func (p *Player) Terminal(keys ...string) string {
	local := "~"
	if len(keys) > 0 {
		local = keys[0]
	}
	return fmt.Sprintf("[%s@ratel %s]# ", strings.TrimSpace(strings.ToLower(p.Name)), local)
}

type Room struct {
	ID      int64 `json:"id"`
	Type    int   `json:"type"`
	State   int   `json:"state"`
	Players int   `json:"players"`
	Robots  int   `json:"robots"`
	Creator int64 `json:"creator"`
}

type Game struct {
	ID         int64                  `json:"id"`
	Type       int                    `json:"type"`
	Pokers     map[int64]model.Pokers `json:"pokers"`
	Almighty   model.Pokers           `json:"almighty"`
	Additional model.Pokers           `json:"pocket"`
	Multiple   int                    `json:"multiple"`
}
