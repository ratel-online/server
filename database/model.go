package database

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/core/util/arrays"
	"github.com/ratel-online/core/util/json"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
)

const initialRune = 'A'

type runeSequence struct {
	currentRune rune
}

func (s *runeSequence) next() rune {
	if s.currentRune == 0 {
		s.currentRune = initialRune
	}
	currentRune := s.currentRune
	s.currentRune++
	return currentRune
}

type Player struct {
	ID     int64  `json:"id"`
	IP     string `json:"ip"`
	Name   string `json:"name"`
	Score  int64  `json:"score"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	RoomID int64  `json:"roomId"`

	conn   *network.Conn
	data   chan *protocol.Packet
	read   bool
	state  consts.StateID
	online bool
}

func (p *Player) Write(bytes []byte) error {
	return p.conn.Write(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) IsOnline() bool {
	return p.online
}

func (p *Player) Offline() {
	p.online = false
	_ = p.conn.Close()
	close(p.data)
	room := getRoom(p.RoomID)
	if room != nil {
		room.Lock()
		defer room.Unlock()
		broadcast(room, fmt.Sprintf("%s lost connection! \n", p.Name))
		if room.State == consts.RoomStateWaiting {
			leaveRoom(room, p)
		}
		roomCancel(room)
	}
}

func (p *Player) Listening() error {
	for {
		pack, err := p.conn.Read()
		if err != nil {
			log.Error(err)
			return err
		}
		if p.read {
			p.data <- pack
		}
	}
}

// 向客户端发生消息
func (p *Player) WriteString(data string) error {
	time.Sleep(30 * time.Millisecond)
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
	if packet == nil {
		return nil, consts.ErrorsChanClosed
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

func (p *Player) AskForStringWithoutTransaction(timeout ...time.Duration) (string, error) {
	packet, err := p.askForPacket(timeout...)
	if err != nil {
		return "", err
	}
	return packet.String(), nil
}

func (p *Player) StartTransaction() {
	p.read = true
	_ = p.WriteString(consts.IsStart)
}

func (p *Player) StopTransaction() {
	p.read = false
	_ = p.WriteString(consts.IsStop)
}

func (p *Player) State(s consts.StateID) {
	p.state = s
}

func (p *Player) GetState() consts.StateID {
	return p.state
}

func (p *Player) Conn(conn *network.Conn) {
	p.conn = conn
	p.data = make(chan *protocol.Packet, 8)
	p.online = true
}

func (p Player) Model() model.Player {
	modelPlayer := model.Player{
		ID:    p.ID,
		Name:  p.Name,
		Score: p.Score,
	}
	room := getRoom(p.RoomID)
	if room != nil && room.Game != nil {
		game := room.Game.(*Game)
		modelPlayer.Pokers = len(game.Pokers[p.ID])
		modelPlayer.Group = game.Groups[p.ID]
	}
	return modelPlayer
}

func (p Player) String() string {
	return fmt.Sprintf("%s[%d]", p.Name, p.ID)
}

type RoomGame interface {
	delete()
}

type Room struct {
	sync.Mutex

	ID                int64     `json:"id"`
	Type              int       `json:"type"`
	Game              RoomGame  `json:"gameId"`
	State             int       `json:"state"`
	Players           int       `json:"players"`
	Banker            int       `json:"banker"`
	Robots            int       `json:"robots"`
	Creator           int64     `json:"creator"`
	ActiveTime        time.Time `json:"activeTime"`
	MaxPlayers        int       `json:"maxPlayers"`
	Password          string    `json:"password"`
	EnableChat        bool      `json:"enableChat"`
	EnableLaiZi       bool      `json:"enableLaiZi"`
	EnableSkill       bool      `json:"enableSkill"`
	EnableLandlord    bool      `json:"enableLandlord"`
	EnableDontShuffle bool      `json:"enableDontShuffle"`
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
	Room        *Room                   `json:"room"`
	Players     []int64                 `json:"players"`
	Groups      map[int64]int           `json:"groups"`
	States      map[int64]chan int      `json:"states"`
	Pokers      map[int64]model.Pokers  `json:"pokers"`
	Universals  []int                   `json:"universals"`
	Decks       int                     `json:"decks"`
	Additional  model.Pokers            `json:"pocket"`
	Multiple    int                     `json:"multiple"`
	FirstPlayer int64                   `json:"firstPlayer"`
	LastPlayer  int64                   `json:"lastPlayer"`
	Robs        []int64                 `json:"robs"`
	FirstRob    int64                   `json:"firstRob"`
	LastRob     int64                   `json:"lastRob"`
	FinalRob    bool                    `json:"finalRob"`
	LastFaces   *model.Faces            `json:"lastFaces"`
	LastPokers  model.Pokers            `json:"lastPokers"`
	Mnemonic    map[int]int             `json:"mnemonic"`
	Skills      map[int64]int           `json:"skills"`
	PlayTimes   map[int64]int           `json:"playTimes"`
	PlayTimeOut map[int64]time.Duration `json:"playTimeOut"`
	Rules       poker.Rules             `json:"rules"`
	Discards    model.Pokers            `json:"discards"`
}

func (game *Game) delete() {
	if game != nil {
		for _, state := range game.States {
			close(state)
		}
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

func (g Game) Team(playerId int64) string {
	if !g.Room.EnableLandlord {
		return "team" + strconv.Itoa(g.Groups[playerId])
	} else {
		if !g.IsLandlord(playerId) {
			return "peasant"
		} else {
			return "landlord"
		}
	}
}
