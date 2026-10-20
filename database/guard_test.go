package database

import (
	"strings"
	"testing"
	"time"

	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
)

// stubRWC 是只提供 IP 的假连接，用来在 database 包内构造带 IP 的玩家。
type stubRWC struct{ ip string }

func (s stubRWC) Read() (*protocol.Packet, error) { select {} }
func (s stubRWC) Write(protocol.Packet) error     { return nil }
func (s stubRWC) Close() error                    { return nil }
func (s stubRWC) IP() string                      { return s.ip }

func newStubConn(ip string) *network.Conn {
	return network.Wrapper(stubRWC{ip: ip})
}

// TestMaskProfanity 归一化后再匹配：空格、标点、全角、大小写、同音字都要挡住
func TestMaskProfanity(t *testing.T) {
	cases := []struct {
		in   string
		want string
		hits int
	}{
		{"傻逼", "**", 1},
		{"傻 逼", "* *", 1}, // 中间插空格，空格保留
		{"傻.逼", "*.*", 1}, // 中间插标点
		{"s b", "* *", 1}, // 拼音加空格
		{"sb", "**", 1},
		{"SB", "**", 1}, // 大小写
		{"ｓｂ", "**", 1}, // 全角
		{"煞笔", "**", 1}, // 同音
		{"沙比", "**", 1},
		{"你妈逼", "***", 1},
		{"asb", "asb", 0}, // 词中间不算
		{"sbx", "sbx", 0},
		{"傻子", "傻子", 0}, // 只是骂人词的一部分，不误伤
		{"今天手气不错", "今天手气不错", 0},
	}
	for _, c := range cases {
		got, hits := maskProfanity(c.in)
		if hits != c.hits {
			t.Fatalf("maskProfanity(%q) hits = %d, want %d (got %q)", c.in, hits, c.hits, got)
		}
		if c.hits > 0 && got != c.want {
			t.Fatalf("maskProfanity(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeText 归一化：全角转半角、去分隔符、混淆字符折叠
func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		"ＳＢ":  "sb",
		"傻 逼": "傻逼",
		"傻.逼": "傻逼",
		"SB!": "sbi",
		"5b":  "sb", // 5 折叠成 s，挡住 "5b" 这类变体
		"ＡＢＣ": "abc",
	}
	for in, want := range cases {
		if got := normalizeText(in); got != want {
			t.Fatalf("normalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameMasksProfanity(t *testing.T) {
	// 全是脏话的名字整段变掩码，再退回默认名
	if got := sanitizeName("傻 逼"); got != defaultName {
		t.Fatalf("sanitizeName = %q, want %q", got, defaultName)
	}
	if got := sanitizeName("煞笔"); got != defaultName {
		t.Fatalf("sanitizeName = %q, want %q", got, defaultName)
	}
	// 只有一部分是脏话时保留其余内容
	if got := sanitizeName("小王煞笔"); got != "小王**" || !strings.Contains(got, "*") {
		t.Fatalf("sanitizeName = %q, want %q", got, "小王**")
	}
}

// TestStrikeEscalation 3 次禁言，累计 6 次踢房 + IP 封禁
func TestStrikeEscalation(t *testing.T) {
	p := &Player{ID: 5001, IP: "9.9.9.1", RoomID: 0}
	for i := 1; i <= 3; i++ {
		muted, kicked := Strike(p, "test")
		if i < 3 && (muted || kicked) {
			t.Fatalf("第 %d 次违规就处罚了", i)
		}
		if i == 3 && !muted {
			t.Fatal("第 3 次违规未触发禁言")
		}
	}
	if d := MutedFor(p); d <= 0 || d > autoMuteDuration {
		t.Fatalf("禁言时长异常: %v", d)
	}
	// 再 3 次 -> 踢房 + IP 封禁
	kicked := false
	for i := 0; i < 3; i++ {
		if _, k := Strike(p, "test"); k {
			kicked = true
		}
	}
	if !kicked {
		t.Fatal("累计 6 次违规未触发踢房")
	}
	if banned, remain := IsIPBanned("9.9.9.1"); !banned || remain <= 0 {
		t.Fatalf("IP 未被封禁: banned=%v remain=%v", banned, remain)
	}
	// 封禁期内换名字也进不来
	if _, _, _, err := claimIdentity(newStubConn("9.9.9.1"), "换名字"); err == nil {
		t.Fatal("封禁 IP 换名字仍然进入了")
	}
}

func TestManualMuteAndUnmute(t *testing.T) {
	p := &Player{ID: 5002, IP: "9.9.9.2"}
	MutePlayer(p.ID, time.Minute)
	if MutedFor(p) <= 0 {
		t.Fatal("手动禁言未生效")
	}
	if !UnmutePlayer(p.ID) {
		t.Fatal("解除禁言应返回 true")
	}
	if MutedFor(p) > 0 {
		t.Fatal("解除禁言后仍然被禁言")
	}
	if UnmutePlayer(p.ID) {
		t.Fatal("重复解除禁言应返回 false")
	}
}

// TestConnectionLimits 同 IP 最多 3 条连接，第 4 条拒绝；顶号不受名额限制
func TestConnectionLimits(t *testing.T) {
	ip := "7.7.7.1"
	for i := 0; i < consts.MaxConnsPerIP; i++ {
		if err := AllowConnection(ip, false); err != nil {
			t.Fatalf("第 %d 条连接被拒绝: %v", i+1, err)
		}
	}
	if err := AllowConnection(ip, false); err != consts.ErrorsTooManyConns {
		t.Fatalf("超出连接上限未被拒绝: %v", err)
	}
	// 顶号（同 IP 同名重连）放行
	if err := AllowConnection(ip, true); err != nil {
		t.Fatalf("顶号被连接上限挡住: %v", err)
	}
	// 此时共占 4 个名额，释放两个后应该又能进
	ReleaseConnection(ip)
	ReleaseConnection(ip)
	if err := AllowConnection(ip, false); err != nil {
		t.Fatalf("释放名额后仍被拒绝: %v", err)
	}
	for i := 0; i < 20; i++ {
		ReleaseConnection(ip)
	}
	if err := AllowConnection(ip, false); err != nil {
		t.Fatalf("全部释放后仍被拒绝: %v", err)
	}
	ReleaseConnection(ip)
}

// TestConnectionRateLimit 同 IP 每分钟最多 10 次连接尝试
func TestConnectionRateLimit(t *testing.T) {
	ip := "7.7.7.2"
	for i := 0; i < consts.ConnRateLimit; i++ {
		if err := AllowConnection(ip, true); err != nil {
			t.Fatalf("第 %d 次尝试被拒绝: %v", i+1, err)
		}
	}
	if err := AllowConnection(ip, true); err != consts.ErrorsConnectTooFast {
		t.Fatalf("超出连接频率未被拒绝: %v", err)
	}
}

// TestWaitingIdle 等待房间 5 分钟无输入
func TestWaitingIdle(t *testing.T) {
	p := &Player{ID: 5003}
	p.Touch()
	if p.IdleFor() > time.Second {
		t.Fatalf("刚活动过就判定挂机: %v", p.IdleFor())
	}
	p.mu.Lock()
	p.lastActive = time.Now().Add(-consts.WaitingIdleTimeout - time.Second)
	p.mu.Unlock()
	if p.IdleFor() <= consts.WaitingIdleTimeout {
		t.Fatalf("超时后仍未判定挂机: %v", p.IdleFor())
	}
}

// TestBotInput 断线后：重连窗口内按正常超时处理，窗口结束后按托管节奏接管
func TestBotInput(t *testing.T) {
	room := CreateRoom(1, consts.GameTypeClassic)
	defer deleteRoom(room)
	room.State = consts.RoomStateRunning

	p := &Player{ID: 5004, RoomID: room.ID, Role: RolePlayer}
	p.mu.Lock()
	p.online = false
	p.offlineAt = time.Now().Add(-consts.ReconnectWindow - time.Second)
	p.mu.Unlock()

	start := time.Now()
	handled, err := p.botInput(time.Second)
	if !handled || err != consts.ErrorsTimeout {
		t.Fatalf("托管未接管: handled=%v err=%v", handled, err)
	}
	if d := time.Since(start); d < consts.BotPace {
		t.Fatalf("托管出牌太快（会瞬间打完一局）: %v", d)
	}

	// 窗口内：不应该立刻代打，最多等到本轮超时
	p.mu.Lock()
	p.offlineAt = time.Now()
	p.mu.Unlock()
	start = time.Now()
	handled, err = p.botInput(50 * time.Millisecond)
	if !handled || err != consts.ErrorsTimeout {
		t.Fatalf("重连窗口内应等待本轮超时: handled=%v err=%v", handled, err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("重连窗口内等待过久: %v", d)
	}

	// 在线时不接管
	p.mu.Lock()
	p.online = true
	p.mu.Unlock()
	if handled, _ := p.botInput(time.Second); handled {
		t.Fatal("在线玩家不应被托管接管")
	}
}

// TestReadyWindow 满员才进入 ready 倒计时，全员 ready 才算齐
func TestReadyWindow(t *testing.T) {
	room := CreateRoom(1, consts.GameTypeClassic)
	defer deleteRoom(room)
	room.MaxPlayers = 2
	seated := getRoomPlayers(room.ID)
	seated[1] = true
	room.Players = 1

	SyncReady(room)
	if info := ReadyStatus(room, 1); info.Required {
		t.Fatal("未满员就要求 ready")
	}

	seated[2] = true
	room.Players = 2
	SyncReady(room)
	info := ReadyStatus(room, 1)
	if !info.Required || info.Ready || info.AllReady {
		t.Fatalf("满员后应要求 ready: %+v", info)
	}
	if got := SetReady(room, 1); got.ReadyNum != 1 || got.AllReady {
		t.Fatalf("ready 计数不对: %+v", got)
	}
	if got := SetReady(room, 2); !got.AllReady {
		t.Fatalf("全员 ready 后 AllReady 应为 true: %+v", got)
	}

	// 旁观者 ready 不算数
	if got := SetReady(room, 99); got.ReadyNum != 2 || !got.AllReady {
		t.Fatalf("旁观者的 ready 被算进去了: %+v", got)
	}

	// 人数回落则清空 ready 窗口
	room.Players = 1
	delete(seated, 2)
	SyncReady(room)
	if info := ReadyStatus(room, 1); info.Required || len(room.Ready) != 0 {
		t.Fatalf("人数回落后 ready 状态未清空: %+v", info)
	}
}

// TestResumeStateKeepsSeat 牌局进行中掉线重连，回到牌桌而不是大厅
func TestResumeStateKeepsSeat(t *testing.T) {
	room := CreateRoom(1, consts.GameTypeClassic)
	defer deleteRoom(room)
	p := &Player{ID: 7001, RoomID: room.ID, Role: RolePlayer}
	getRoomPlayers(room.ID)[p.ID] = true
	room.Players = 1
	room.State = consts.RoomStateRunning

	if got := ResumeState(p); got != consts.StateGame {
		t.Fatalf("断线重连应回到牌桌, got %v", got)
	}
	// 不在房间里的玩家回大厅
	p2 := &Player{ID: 7002, RoomID: room.ID, Role: RolePlayer}
	if got := ResumeState(p2); got != consts.StateWelcome {
		t.Fatalf("不在座位上的玩家应回大厅, got %v", got)
	}
}
