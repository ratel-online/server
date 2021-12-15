package service

import (
	"fmt"
	constx "github.com/ratel-online/core/consts"
	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/core/util/arrays"
	"github.com/ratel-online/core/util/async"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/server/consts"
	"strings"
	"sync"
	"time"
)

type Player struct {
	conn     *network.Conn
	online   bool
	channels map[int]chan *model.Req

	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Score  int64  `json:"score"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	RoomID int64  `json:"roomId"`
}

func NewPlayer(conn *network.Conn) *Player {
	player := &Player{}
	player.conn = conn
	player.online = true
	player.channels = map[int]chan *model.Req{}
	for i := 1; i <= 3; i++ {
		player.channels[i] = make(chan *model.Req, 10)
	}
	connPlayers.Set(conn.ID(), player)
	return player
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) Auth() error {
	authChan := make(chan *model.AuthInfo)
	defer close(authChan)
	async.Async(func() {
		packet, err := p.conn.Read()
		if err != nil {
			log.Error(err)
			return
		}
		authInfo := &model.AuthInfo{}
		err = packet.Unmarshal(authInfo)
		if err != nil {
			log.Error(err)
			return
		}
		authChan <- authInfo
	})
	select {
	case authInfo := <-authChan:
		p.ID = authInfo.ID
		p.Name = authInfo.Name
		p.Score = authInfo.Score
		log.Infof("player auth accessed, %d:%s\n", authInfo.ID, authInfo.Name)
		return nil
	case <-time.After(3 * time.Second):
		return consts.ErrorsAuthFail
	}
}

func (p *Player) Offline() {
	p.online = false
	_ = p.conn.Close()
	for _, c := range p.channels {
		close(c)
	}
	room := getRoom(p.RoomID)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		broadcast(room.ID, fmt.Sprintf("%s lost connection!\n", p.Name))
		if room.State == consts.RoomStateWaiting {
			_leaveRoom(room, p)
		}
		roomCancel(room)
	}
}

func (p *Player) Listening() error {
	async.Async(func() {
		var req *model.Req
		for {
			req = <-p.channels[constx.Service]
			if req == nil {
				break
			}
			_ = p.WriteObject(servlets.handle(p, *req))
		}
	})
	for {
		packet, err := p.conn.Read()
		if err != nil {
			log.Error(err)
			return err
		}
		req := &model.Req{}
		err = packet.Unmarshal(req)
		if err != nil {
			log.Error(err)
			return err
		}
		if c, ok := p.channels[req.Type]; ok {
			if len(c) < cap(c) {
				c <- req
			}
		}
	}
}

func (p *Player) WriteString(data string) error {
	return p.conn.Write(protocol.Packet{
		Body: []byte(data),
	})
}

func (p *Player) WriteObject(data interface{}) error {
	return p.conn.Write(protocol.Packet{
		Body: json.Marshal(data),
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
	p.StartTransaction()
	defer p.StopTransaction()
	return p.askForPacket(timeout...)
}

func (p *Player) askForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	var req *model.Req
	if len(timeout) > 0 {
		select {
		case req = <-p.channels[constx.Instruct]:
		case <-time.After(timeout[0]):
			return nil, consts.ErrorsTimeout
		}
	} else {
		req = <-p.channels[constx.Instruct]
	}
	if req == nil {
		return nil, consts.ErrorsChanClosed
	}
	single := strings.ToLower(string(req.Data))
	if single == "exit" || single == "e" {
		return nil, consts.ErrorsExist
	}
	panic("todo")
	return nil, nil
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

func (p *Player) AskForStringWithoutTransaction(timeout ...time.Duration) (string, error) {
	packet, err := p.askForPacket(timeout...)
	if err != nil {
		return "", err
	}
	return packet.String(), nil
}

func (p *Player) StartTransaction() {
	_ = p.WriteString(consts.IsStart)
}

func (p *Player) StopTransaction() {
	_ = p.WriteString(consts.IsStop)
}

func (p *Player) State(s consts.StateID) {
}

func (p *Player) GetState() consts.StateID {
	return 0
}

func (p Player) Model() model.Player {
	modelPlayer := model.Player{
		ID:    p.ID,
		Name:  p.Name,
		Score: p.Score,
	}
	return modelPlayer
}

func (p Player) String() string {
	return fmt.Sprintf("%s[%d]", p.Name, p.ID)
}

type Room struct {
	sync.Mutex

	ID      int64 `json:"id"`
	Type    int   `json:"type"`
	Game    *Game `json:"gameId"`
	State   int   `json:"state"`
	Players int   `json:"players"`
	Robots  int   `json:"robots"`
	Creator int64 `json:"creator"`
}

func (r *Room) Model() model.Room {
	return model.Room{
		ID:        r.ID,
		Type:      r.Type,
		TypeDesc:  consts.GameTypes[r.Type],
		Players:   r.Players,
		State:     r.State,
		StateDesc: consts.RoomStates[r.State],
		Creator:   r.Creator,
	}
}

type Game struct {
	Players     []int64                `json:"players"`
	Groups      map[int64]int          `json:"groups"`
	States      map[int64]chan int     `json:"states"`
	Pokers      map[int64]model.Pokers `json:"pokers"`
	Universals  []int                  `json:"universals"`
	Additional  model.Pokers           `json:"additional"`
	Multiple    int                    `json:"multiple"`
	FirstPlayer int64                  `json:"firstPlayer"`
	LastPlayer  int64                  `json:"lastPlayer"`
	FirstRob    int64                  `json:"firstRob"`
	LastRob     int64                  `json:"lastRob"`
	FinalRob    bool                   `json:"finalRob"`
	LastFaces   *model.Faces           `json:"lastFaces"`
	LastPokers  model.Pokers           `json:"lastPokers"`
	Mnemonic    map[int]int            `json:"mnemonic"`
}

func (g Game) Model() model.Game {
	modelPokers := map[int64]int{}
	for id, pokers := range g.Pokers {
		modelPokers[id] = len(pokers)
	}
	modelLastPokers := make([]int, 0)
	if len(g.LastPokers) > 0 {
		modelLastPokers = g.LastPokers.Keys()
	}
	modelAdditional := make([]int, 0)
	if len(g.Additional) > 0 {
		modelAdditional = g.Additional.Keys()
	}
	return model.Game{
		Players:    g.Players,
		Pokers:     modelPokers,
		Groups:     g.Groups,
		Mnemonic:   g.Mnemonic,
		LastPokers: modelLastPokers,
		LastPlayer: g.LastPlayer,
		Universals: g.Universals,
		Additional: modelAdditional,
	}
}

func (g Game) NextPlayer(curr int64) int64 {
	idx := arrays.IndexOf(g.Players, curr)
	return g.Players[(idx+1)%len(g.Players)]
}

func (g Game) PrevPlayer(curr int64) int64 {
	idx := arrays.IndexOf(g.Players, curr)
	return g.Players[(idx+len(g.Players))%len(g.Players)]
}

func (g Game) IsTeammate(player1, player2 int64) bool {
	return g.Groups[player1] == g.Groups[player2]
}

func (g Game) IsLandlord(playerId int64) bool {
	return g.Groups[playerId] == 1
}
