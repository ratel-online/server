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

// Seated 表示玩家是否占着一个牌座（房主与普通玩家都算，旁观不算）。
func (r Role) Seated() bool {
	return r == RolePlayer || r == RoleOwner
}

// ConnState 代表玩家的一次连接（"一代"）。
// 顶号时 Player 会绑定新的 ConnState，旧一代通过 done 收到通知后安静退出，
// 既不会关掉新连接，也不会把玩家从房间/牌局里带走。
type ConnState struct {
	conn   *network.Conn
	data   chan *protocol.Packet
	done   chan struct{}
	once   sync.Once
	ip     string
	ipOnce sync.Once
}

func NewConnState(conn *network.Conn) *ConnState {
	return &ConnState{
		conn: conn,
		data: make(chan *protocol.Packet, 8),
		done: make(chan struct{}),
		ip:   normalizeIP(conn.IP()),
	}
}

// close 通知这一代连接失效，可重复调用。
func (cs *ConnState) close() {
	cs.once.Do(func() { close(cs.done) })
}

// releaseIP 归还该 IP 的连接名额，一代连接只归还一次。
func (cs *ConnState) releaseIP() {
	cs.ipOnce.Do(func() { ReleaseConnection(cs.ip) })
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

	mu         sync.Mutex
	cs         *ConnState // 当前这一代连接；顶号会整体替换
	read       bool
	state      consts.StateID
	online     bool
	runGen     int64     // 状态机代际：顶号后旧状态机据此退出，避免两个协程抢同一个输入
	baseName   string    // 不含 "#短ID" 后缀的基础名
	nameKey    string    // 同 IP 同名的排他键
	lastActive time.Time // 最近一次有输入的时间，用于挂机检测
	offlineAt  time.Time // 断线时间，用于重连窗口与托管
	retired    bool      // 已被顶号作废，旧协程见到后立刻退出
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
	p.retired = true // 让仍在这一代跑的状态机在下次输入时立刻退出
	p.runGen++       // 让仍在这一代跑的状态机退出
	p.online = false
	p.cs = nil
	p.read = false
	p.mu.Unlock()

	if old != nil {
		old.close()
		old.releaseIP()
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

// amountMu 保护积分读写（Player 会被按值复制，所以用包级锁）
var amountMu sync.Mutex

// Points 当前积分
func (p *Player) Points() int {
	amountMu.Lock()
	defer amountMu.Unlock()
	return int(p.Amount)
}

// AddPoints 增减积分，返回实际变动量；扣分不会把积分扣成负数
func (p *Player) AddPoints(delta int) int {
	amountMu.Lock()
	defer amountMu.Unlock()
	if delta >= 0 {
		p.Amount += uint(delta)
		return delta
	}
	pay := -delta
	if pay > int(p.Amount) {
		pay = int(p.Amount)
	}
	p.Amount -= uint(pay)
	return -pay
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
	cs.releaseIP()

	p.mu.Lock()
	current := p.cs == cs
	if current {
		p.online = false
		p.cs = nil
		p.read = false
		p.offlineAt = time.Now()
	}
	p.mu.Unlock()

	if !current {
		log.Infof("%s[%d] old session closed, newer connection is serving.\n", p.BaseName(), p.ID)
		return
	}

	_ = cs.conn.Close()
	room := getRoom(p.RoomID)

	// 牌局进行中掉线：保留座位与名字，重连窗口内回来还是同一个人，
	// 超时后由托管接管（见 botInput），不把牌局打散。
	if room != nil && room.State == consts.RoomStateRunning {
		room.Lock()
		defer room.Unlock()
		broadcast(room, fmt.Sprintf("%s lost connection! seat is kept for %d seconds.\n",
			p.Name, int(consts.ReconnectWindow.Seconds())))
		return
	}

	releaseName(p) // 释放名字占用，避免名字被离线玩家永久锁死
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

// Touch 记录一次玩家活动（收到输入），用于挂机检测。
func (p *Player) Touch() {
	p.mu.Lock()
	p.lastActive = time.Now()
	p.mu.Unlock()
}

// IdleFor 返回距离最近一次活动过了多久。
func (p *Player) IdleFor() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastActive.IsZero() {
		return 0
	}
	return time.Since(p.lastActive)
}

// OfflineFor 返回断线了多久，在线返回 0。
func (p *Player) OfflineFor() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.online || p.offlineAt.IsZero() {
		return 0
	}
	return time.Since(p.offlineAt)
}

// IsRetired 判断该对象是否已被顶号作废。
func (p *Player) IsRetired() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.retired
}

// HandBackIfRetired 在被顶号时把"刚收到的一手"交还给同一个通道，
// 让接管座位的新会话接着走，避免旧协程替新玩家出牌。
// 返回 true 表示本代已作废，调用方应直接退出。
func (p *Player) HandBackIfRetired(ch chan int, state int) (retired bool) {
	if !p.IsRetired() {
		return false
	}
	// 房间可能已经被清理、通道已经关闭，这时交还失败也不影响什么
	defer func() { _ = recover() }()
	select {
	case ch <- state:
	default:
	}
	return true
}

// CanBotContinue 判断断线玩家是否应该留在牌局里由托管接手：
// 同一代状态机、已断线、仍在牌座上、房间还在进行中。
func (p *Player) CanBotContinue(gen int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.runGen != gen || p.online || p.retired {
		return false
	}
	if p.offlineAt.IsZero() || p.RoomID == 0 || !p.Role.Seated() {
		return false
	}
	room := getRoom(p.RoomID)
	return room != nil && room.State == consts.RoomStateRunning
}

// roomRunning 判断房间是否处于牌局进行中（加锁读取，避免与开局时的写入竞争）。
func roomRunning(room *Room) bool {
	if room == nil {
		return false
	}
	room.Lock()
	defer room.Unlock()
	return room.State == consts.RoomStateRunning
}

// botInput 处理"已断线但牌局还在进行"的输入：
//   - 重连窗口内：最多等到本轮超时，期间对方回来就交还控制权；
//   - 窗口结束后：按托管节奏停顿一下，返回超时，交给上层自动出牌。
//
// 返回 handled=true 表示本次输入已由托管处理，调用方不要再读 socket。
func (p *Player) botInput(timeout time.Duration) (bool, error) {
	p.mu.Lock()
	online, retired, offlineAt, role := p.online, p.retired, p.offlineAt, p.Role
	p.mu.Unlock()
	if online || retired || offlineAt.IsZero() || !role.Seated() {
		return false, nil
	}
	room := getRoom(p.RoomID)
	if !roomRunning(room) {
		return false, nil
	}

	if wait := consts.ReconnectWindow - time.Since(offlineAt); wait > 0 {
		capped := wait
		if timeout > 0 && timeout < capped {
			capped = timeout
		}
		// 边等边看：本人重连（新会话）或被顶号时立刻交还控制权
		deadline := time.Now().Add(capped)
		for {
			remain := time.Until(deadline)
			if remain <= 0 {
				break
			}
			if remain > 200*time.Millisecond {
				remain = 200 * time.Millisecond
			}
			time.Sleep(remain)
			if p.IsOnline() || p.IsRetired() {
				return false, nil
			}
		}
		if capped < wait {
			// 本轮超时了，但重连窗口还在：按普通超时处理，不额外停顿
			return true, consts.ErrorsTimeout
		}
	}
	time.Sleep(consts.BotPace)
	if p.IsRetired() {
		return true, consts.ErrorsChanClosed
	}
	return true, consts.ErrorsTimeout
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
	return p.readInput(timeout...)
}

// readInput 是所有输入的统一入口：先判断是否该由托管接手，再正常读连接。
func (p *Player) readInput(timeout ...time.Duration) (*protocol.Packet, error) {
	var limit time.Duration
	if len(timeout) > 0 {
		limit = timeout[0]
	}
	if handled, err := p.botInput(limit); handled {
		return nil, err
	}
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
	p.Touch() // 有输入就刷新活跃时间，等待房间据此判断挂机
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
	packet, err := p.readInput(timeout...)
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
	p.retired = false
	p.offlineAt = time.Time{}
	p.lastActive = time.Now()
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
	if room != nil {
		// 只有斗地主类牌局才有 Pokers / Groups 信息，其它模式安全跳过
		if game, ok := room.Game.(*Game); ok && game != nil {
			modelPlayer.Pokers = len(game.Pokers[p.ID])
			modelPlayer.Group = game.Groups[p.ID]
		}
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

	ID                  int64          `json:"id"`
	Type                int            `json:"type"`
	Game                RoomGame       `json:"gameId"`
	State               int            `json:"state"`
	Players             int            `json:"players"`
	Banker              int            `json:"banker"`
	Robots              int            `json:"robots"`
	Creator             int64          `json:"creator"`
	ActiveTime          time.Time      `json:"activeTime"`
	MaxPlayers          int            `json:"maxPlayers"`
	Password            string         `json:"password"`
	EnableChat          bool           `json:"enableChat"`
	EnableLaiZi         bool           `json:"enableLaiZi"`
	EnableSkill         bool           `json:"enableSkill"`
	EnableLandlord      bool           `json:"enableLandlord"`
	EnableDontShuffle   bool           `json:"enableDontShuffle"`
	EnableShowIP        bool           `json:"enableShowIP"`
	EnableJokerAsTarget bool           `json:"enableJokerAsTarget"`
	UndercoverNum       int            `json:"undercoverNum"` // 卧底数量
	BlankWordMode       bool           `json:"blankWordMode"` // 空白词模式
	Ready               map[int64]bool `json:"ready"`         // 满员后的 ready 状态
	ReadyUntil          time.Time      `json:"readyUntil"`    // ready 倒计时截止
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
