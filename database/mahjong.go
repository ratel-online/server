package database

import (
	"sync"
	"time"

	"github.com/ratel-online/server/mahjong"
)

// 牌桌内部状态信号
const (
	MahjongStateWaiting = 1
)

// Mahjong 一局大众麻将的运行时数据。
//
// 与斗地主等模式不同，麻将每个回合需要依次询问多名玩家（吃碰杠胡），
// 因此整局由一个“桌长”协程串行驱动：其余玩家只在状态通道上等待，
// 收到 MahjongStateWaiting 即表示本局结束。
type Mahjong struct {
	Room   *Room              `json:"room"`
	Seats  []int64            `json:"seats"`
	States map[int64]chan int `json:"states"`
	Game   *mahjong.Game      `json:"game"`

	mu      sync.Mutex
	driver  int64
	started bool
	arrived map[int64]bool
	quits   map[int64]bool
	done    bool
	ended   bool
	finish  sync.Once
}

// MarkQuit 记录中途退出牌局的玩家
func (m *Mahjong) MarkQuit(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.quits == nil {
		m.quits = map[int64]bool{}
	}
	m.quits[id] = true
}

// HasQuit 玩家是否中途退出
func (m *Mahjong) HasQuit(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.quits[id]
}

// MarkEnded 结算只执行一次
func (m *Mahjong) MarkEnded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ended {
		return false
	}
	m.ended = true
	return true
}

// Clean 房间被回收时确保所有玩家都能退出牌局
func (m *Mahjong) Clean() {
	m.Finish()
}

// Finish 通知所有玩家本局结束（幂等）
func (m *Mahjong) Finish() {
	if m == nil {
		return
	}
	m.finish.Do(func() {
		m.mu.Lock()
		m.done = true
		chans := make([]chan int, 0, len(m.States))
		for _, ch := range m.States {
			chans = append(chans, ch)
		}
		m.mu.Unlock()
		for _, ch := range chans {
			select {
			case ch <- MahjongStateWaiting:
			default:
			}
		}
	})
}

// Finished 本局是否已经结束
func (m *Mahjong) Finished() bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done
}

// Arrive 玩家进入牌桌。第一个进入的玩家成为桌长并负责驱动整局。
func (m *Mahjong) Arrive(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.States[id]; !ok {
		return false
	}
	if m.arrived == nil {
		m.arrived = make(map[int64]bool, len(m.States))
	}
	if m.driver == 0 {
		m.driver = id
	}
	m.arrived[id] = true
	if m.driver != id || m.started {
		return false
	}
	// 整局只启动一次
	m.started = true
	return true
}

// WaitArrive 等待所有座位就绪
func (m *Mahjong) WaitArrive(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		m.mu.Lock()
		n := len(m.arrived)
		m.mu.Unlock()
		if n >= len(m.Seats) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Seated 指定玩家是否在本局座位中
func (m *Mahjong) Seated(id int64) bool {
	_, ok := m.States[id]
	return ok
}
