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

type Role string

const (
	RolePlayer    Role = "player"
	RoleOwner     Role = "owner"
	RoleSpectator Role = "spectator"
)

// ConnState 代表玩家的一次连接（"一代"）。
// 顶号时 Player 会绑定新的 ConnState，旧一代通过 done 收到通知后安静退出，
// 既不会关掉新连接，也不会把玩家从房间/牌局里带走。
type ConnState struct {
	conn *network.Conn
	data chan *protocol.Packet
	done chan struct{}
	once sync.Once
}

func NewConnState(conn *network.Conn) *ConnState {
	return &ConnState{
		conn: conn,
		data: make(chan *protocol.Packet, 8),
		done: make(chan struct{}),
	}
}

// close 通知这一代连接失效，可重复调用。
func (cs *ConnState) close() {
	cs.once.Do(func() { close(cs.done) })
}

type Player struct {
	ID     int64  `json:"id"`
	IP     string `json:"ip"`
	Name   string `json:"name"`
	Mode   int    `json:"mode"`
	Type   int    `json:"type"`
	Amount uint   `json:"amount"`
	RoomID int64  `json:"roomId"`
	Role   Role   `json:"role"`

	mu       sync.Mutex
	cs       *ConnState // 当前这一代连接；顶号会整体替换
	read     bool
	state    consts.StateID
	online   bool
	runGen   int64  // 状态机代际：顶号后旧状态机据此退出，避免两个协程抢同一个输入
	baseName string // 不含 "#短ID" 后缀的基础名
	nameKey  string // 同 IP 同名的排他键
}

// playerSnapshot 顶号时从旧玩家对象继承的字段（不含锁与连接）。
type playerSnapshot struct {
	ID       int64
	IP       string
	Name     string
	Mode     int
	Type     int
	Amount   uint
	RoomID   int64
	Role     Role
	baseName string
	nameKey  string
}

// snapshot 取一份继承用的字段快照。
func (p *Player) snapshot() playerSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return playerSnapshot{
		ID:       p.ID,
		IP:       p.IP,
		Name:     p.Name,
		Mode:     p.Mode,
		Type:     p.Type,
		Amount:   p.Amount,
		RoomID:   p.RoomID,
		Role:     p.Role,
		baseName: p.baseName,
		nameKey:  p.nameKey,
	}
}

// invalidate 彻底作废旧对象（顶号时使用）：状态机失效、连接关闭。
// 旧协程随后会自行退出，且它的任何写入都落在已关闭的旧 socket 上，不会打扰新会话。
func (p *Player) invalidate() {
	old := p.currentState()
	p.mu.Lock()
	p.runGen++ // 让仍在这一代跑的状态机退出
	p.online = false
	p.cs = nil
	p.read = false
	p.mu.Unlock()

	if old != nil {
		old.close()
		_ = old.conn.Close()
	}
}

// BeginStateRun 开启一代状态机，返回代际号。上一次的协程检测到代际变化后会退出。
func (p *Player) BeginStateRun() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runGen++
	return p.runGen
}

// IsCurrentRun 判断某代状态机是否仍是当前有效的那一个。
func (p *Player) IsCurrentRun(gen int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runGen == gen
}

// currentState 取当前这一代连接。
func (p *Player) currentState() *ConnState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cs
}

func (p *Player) writePacket(packet protocol.Packet) error {
	p.mu.Lock()
	cs := p.cs
	p.mu.Unlock()
	if cs == nil {
		return consts.ErrorsChanClosed
	}
	return cs.conn.Write(packet)
}

func (p *Player) Write(bytes []byte) error {
	return p.writePacket(protocol.Packet{
		Body: bytes,
	})
}

func (p *Player) IsOnline() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.online
}

// Offline 关闭当前连接。被顶号后旧协程应调用 OfflineConn(旧 cs)。
func (p *Player) Offline() {
	p.OfflineConn(p.currentState())
}

// OfflineConn 关闭指定的一代连接。
// 如果这一代已经被顶掉（p.cs 已换成新连接），只通知本代协程退出，
// 房间、名字、余额都留给新连接，不做任何清理。
func (p *Player) OfflineConn(cs *ConnState) {
	if cs == nil {
		return
	}
	cs.close()

	p.mu.Lock()
	current := p.cs == cs
	if current {
		p.online = false
		p.cs = nil
		p.read = false
	}
	p.mu.Unlock()

	if !current {
		log.Infof("%s[%d] old session closed, newer connection is serving.\n", p.BaseName(), p.ID)
		return
	}

	_ = cs.conn.Close()
	releaseName(p) // 释放名字占用，避免名字被离线玩家永久锁死
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

// Listening 读取指定一代连接上的数据包。连接被顶掉后返回。
func (p *Player) Listening(cs *ConnState) error {
	if cs == nil {
		return consts.ErrorsChanClosed
	}
	loopCount := 0
	for {
		loopCount++
		if loopCount%1000 == 0 {
			log.Infof("[Player.Listening] Player %d loop count: %d\n", p.ID, loopCount)
		}
		pack, err := cs.conn.Read()
		if err != nil {
			log.Error(err)
			return err
		}
		p.mu.Lock()
		deliver := p.cs == cs && p.read
		p.mu.Unlock()
		if !deliver {
			continue
		}
		select {
		case cs.data <- pack:
		case <-cs.done:
			return consts.ErrorsChanClosed
		}
	}
}

// 向客户端发生消息
func (p *Player) WriteString(data string) error {
	return p.writePacket(protocol.Packet{
		Body: []byte(data),
	})
}

func (p *Player) WriteObject(data interface{}) error {
	return p.writePacket(protocol.Packet{
		Body: json.Marshal(data),
	})
}

func (p *Player) WriteError(err error) error {
	if err == consts.ErrorsExist {
		return err
	}
	return p.writePacket(protocol.Packet{
		Body: []byte(err.Error() + "\n"),
	})
}

func (p *Player) AskForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	p.StartTransaction()
	defer p.StopTransaction()
	return p.askForPacket(timeout...)
}

func (p *Player) askForPacket(timeout ...time.Duration) (*protocol.Packet, error) {
	cs := p.currentState()
	if cs == nil {
		return nil, consts.ErrorsChanClosed
	}
	var packet *protocol.Packet
	if len(timeout) > 0 {
		timer := time.NewTimer(timeout[0])
		defer timer.Stop()
		select {
		case packet = <-cs.data:
		case <-cs.done: // 顶号/断线：让这一代的回合逻辑立即退出
			return nil, consts.ErrorsChanClosed
		case <-timer.C:
			return nil, consts.ErrorsTimeout
		}
	} else {
		select {
		case packet = <-cs.data:
		case <-cs.done:
			return nil, consts.ErrorsChanClosed
		}
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
	p.mu.Lock()
	p.read = true
	p.mu.Unlock()
	_ = p.WriteString(consts.IsStart)
}

func (p *Player) StopTransaction() {
	p.mu.Lock()
	p.read = false
	p.mu.Unlock()
	_ = p.WriteString(consts.IsStop)
}

func (p *Player) State(s consts.StateID) {
	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
}

func (p *Player) GetState() consts.StateID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// BindConn 绑定一代新连接。若该玩家此前有连接（顶号），旧连接会被通知失效并关闭，
// 但玩家 ID、名字、房间、牌局进度全部保留。
func (p *Player) BindConn(conn *network.Conn) *ConnState {
	p.mu.Lock()
	old := p.cs
	cs := NewConnState(conn)
	p.cs = cs
	p.read = false
	p.online = true
	p.mu.Unlock()

	if old != nil {
		old.close()
		_ = old.conn.Close()
	}
	return cs
}

// Conn 兼容旧调用，等价于 BindConn。
func (p *Player) Conn(conn *network.Conn) {
	p.BindConn(conn)
}

func (p *Player) Model() model.Player {
	modelPlayer := model.Player{
		ID:   p.ID,
		Name: p.Name,
	}
	room := getRoom(p.RoomID)
	if room != nil && room.Game != nil {
		game := room.Game.(*Game)
		modelPlayer.Pokers = len(game.Pokers[p.ID])
		modelPlayer.Group = game.Groups[p.ID]
	}
	return modelPlayer
}

func (p *Player) String() string {
	return fmt.Sprintf("%s[%d]", p.BaseName(), p.ID)
}

type RoomGame interface {
	Clean()
}

type Room struct {
	sync.Mutex

	ID                  int64     `json:"id"`
	Type                int       `json:"type"`
	Game                RoomGame  `json:"gameId"`
	State               int       `json:"state"`
	Players             int       `json:"players"`
	Banker              int       `json:"banker"`
	Robots              int       `json:"robots"`
	Creator             int64     `json:"creator"`
	ActiveTime          time.Time `json:"activeTime"`
	MaxPlayers          int       `json:"maxPlayers"`
	Password            string    `json:"password"`
	EnableChat          bool      `json:"enableChat"`
	EnableLaiZi         bool      `json:"enableLaiZi"`
	EnableSkill         bool      `json:"enableSkill"`
	EnableLandlord      bool      `json:"enableLandlord"`
	EnableDontShuffle   bool      `json:"enableDontShuffle"`
	EnableShowIP        bool      `json:"enableShowIP"`
	EnableJokerAsTarget bool      `json:"enableJokerAsTarget"`
	UndercoverNum       int       `json:"undercoverNum"` // 卧底数量
	BlankWordMode       bool      `json:"blankWordMode"` // 空白词模式
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

func (game *Game) Clean() {
	if game != nil {
		for _, state := range game.States {
			close(state)
		}
	}
}

func (game *Game) Start() {

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
