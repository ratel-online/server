package model

import (
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"strings"
	"time"
)

type Player struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Score  int64  `json:"score"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	RoomID int64  `json:"roomId"`

	conn  *network.Conn
	data  chan *protocol.Packet
	read  bool
	state consts.StateID
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) ReadLoop() {
	for {
		pack, err := p.conn.Read()
		if err != nil {
			log.Error(err)
			return
		}
		if p.read {
			p.data <- pack
			p.read = false
		}
	}
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

func (p *Player) AskForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	err := p.WriteString(consts.IS)
	if err != nil {
		return nil, err
	}
	p.read = true
	var packet *protocol.Packet
	if len(timeout) > 0 {
		select {
		case packet = <-p.data:
		case <-time.After(timeout[0]):
			return nil, consts.ErrorsTimeout
		}
	} else {
		packet = <-p.data
	}
	single := strings.ToLower(packet.String())
	if single == "exit" || single == "e" {
		return nil, consts.ErrorsExist
	}
	return packet, nil
}

func (p *Player) AskForInt(timeout ...time.Duration) (int, error) {
	packet, err := p.AskForPacket(timeout...)
	if err != nil {
		return 0, err
	}
	return packet.Int()
}

func (p *Player) AskForInt64(timeout ...time.Duration) (int64, error) {
	packet, err := p.AskForPacket(timeout...)
	if err != nil {
		return 0, err
	}
	return packet.Int64()
}

func (p *Player) AskForString(timeout ...time.Duration) (string, error) {
	packet, err := p.AskForPacket(timeout...)
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
	p.data = make(chan *protocol.Packet)
}

type Room struct {
	ID      int64 `json:"id"`
	Type    int   `json:"type"`
	Game    *Game `json:"gameId"`
	State   int   `json:"state"`
	Players int   `json:"players"`
	Robots  int   `json:"robots"`
	Creator int64 `json:"creator"`
}

type Game struct {
	Players     []int64                `json:"players"`
	Groups      map[int64]int          `json:"groups"`
	States      map[int64]chan int     `json:"states"`
	Pokers      map[int64]model.Pokers `json:"pokers"`
	Landlord    int64                  `json:"landlord"`
	FirstPlayer int64                  `json:"firstPlayer"`
	Almighty    model.Pokers           `json:"almighty"`
	Additional  model.Pokers           `json:"pocket"`
	Multiple    int                    `json:"multiple"`
}
