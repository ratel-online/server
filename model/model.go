package model

import (
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
)

type Player struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Score int64  `json:"score"`

	room  *Room
	conn  *network.Conn
	state consts.StateID
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) Read() (*protocol.Packet, error){
	return p.conn.Read()
}

func (p *Player) WriteString(data string) error {
	return p.conn.Write(protocol.Packet{
		Body: []byte(data),
	})
}

func (p *Player) WriteError(err error) error {
	return p.conn.Write(protocol.Packet{
		Body: []byte(err.Error()),
	})
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

type Room struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Type    int       `json:"type"`
	Players []*Player `json:"players"`
	GameID  int64     `json:"game_id"`
	Creator string    `json:"creator"`
}

type Game struct {
	ID       int64             `json:"id"`
	State    int               `json:"state"`
	Pokers   map[int64][]Poker `json:"pokers"`
	Multiple int               `json:"multiple"`
}

type Poker struct {
	ID   int `json:"id"`
	Type int `json:"type"`
}
