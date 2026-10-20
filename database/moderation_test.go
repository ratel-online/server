package database

import (
	"testing"
	"time"

	"github.com/ratel-online/server/consts"
)

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"张三", "张三"},
		{"张 三", "张三"},
		{"张\u200b三", "张三"},
		{"Ｚｈａｎｇ", "Zhang"},
		{"admin#0007", "admin0007"}, // '#' 由服务端独占
		{"   ", defaultName},
		{"", defaultName},
		{"一二三四五六七八九十十一十二十三", "一二三四五六七八九十十一"},
	}
	for _, c := range cases {
		if got := sanitizeName(c.in); got != c.want {
			t.Fatalf("sanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestAssignNameNoSuffix 同名不再加编号，靠 "#短ID" 区分
func TestAssignNameNoSuffix(t *testing.T) {
	p1 := &Player{ID: 7, IP: "1.1.1.1", online: true}
	p2 := &Player{ID: 8, IP: "2.2.2.2", online: true}
	if err := assignName(p1, "张三"); err != nil {
		t.Fatalf("首个玩家分配名字失败: %v", err)
	}
	if err := assignName(p2, "张三"); err != nil {
		t.Fatalf("不同 IP 同名应允许: %v", err)
	}

	if want := "张三#" + shortID(7); p1.Name != want {
		t.Fatalf("p1.Name = %q, want %q", p1.Name, want)
	}
	if want := "张三#" + shortID(8); p2.Name != want {
		t.Fatalf("p2.Name = %q, want %q", p2.Name, want)
	}
	if p1.Name == p2.Name {
		t.Fatalf("两个玩家的展示名相同: %q", p1.Name)
	}
	if p1.BaseName() != "张三" || p2.BaseName() != "张三" {
		t.Fatalf("基础名不应带编号: %q / %q", p1.BaseName(), p2.BaseName())
	}
}

// TestSameIPSameNameIsTakeoverTarget 同 IP 同名不再拒绝，而是作为"顶号"目标：
// 新连接接管旧玩家（ID 沿用）。registerName 仍保留排他保护，防止并发下写坏索引。
func TestSameIPSameNameIsTakeoverTarget(t *testing.T) {
	old := &Player{ID: 11, IP: "1.1.1.1", online: true}
	if err := assignName(old, "老王"); err != nil {
		t.Fatalf("首个玩家分配名字失败: %v", err)
	}

	// 换端口也算同一来源
	if got := lookupSameIPName("1.1.1.1:5678", "老王"); got != old {
		t.Fatalf("同 IP 同名应命中顶号目标, got %v", got)
	}
	// 不同 IP 不命中
	if got := lookupSameIPName("2.2.2.2", "老王"); got != nil {
		t.Fatalf("不同 IP 不应命中顶号目标, got %v", got)
	}
	// 同 IP 不同名不命中
	if got := lookupSameIPName("1.1.1.1", "老李"); got != nil {
		t.Fatalf("不同名不应命中顶号目标, got %v", got)
	}

	// 离线后不再是顶号目标，名字可以被重新登记
	old.online = false
	if got := lookupSameIPName("1.1.1.1", "老王"); got != nil {
		t.Fatalf("离线玩家不应是顶号目标, got %v", got)
	}
	releaseName(old)
	rejoin := &Player{ID: 15, IP: "1.1.1.1", online: true}
	if err := assignName(rejoin, "老王"); err != nil {
		t.Fatalf("离线后同名应可重新登记: %v", err)
	}
	if rejoin.Name == old.Name {
		t.Fatalf("离线重进应分配新 ID: %q", rejoin.Name)
	}
}

// TestRegisterNameRejectsConcurrentDuplicate 并发下同一 IP 同名只能登记成功一次。
func TestRegisterNameRejectsConcurrentDuplicate(t *testing.T) {
	a := &Player{ID: 21, IP: "3.3.3.3", online: true}
	if err := assignName(a, "并发"); err != nil {
		t.Fatalf("登记失败: %v", err)
	}
	b := &Player{ID: 22, IP: "3.3.3.3", online: true}
	if err := assignName(b, "并发"); err != consts.ErrorsNameInUse {
		t.Fatalf("同 IP 同名重复登记未被拒绝, got %v", err)
	}
	releaseName(a)
}

// TestResumeState 顶号后：等待房间原地恢复，牌局进行中退回大厅。
func TestResumeState(t *testing.T) {
	p := &Player{}
	if got := ResumeState(p); got != consts.StateWelcome {
		t.Fatalf("无房间应回大厅, got %v", got)
	}

	room := CreateRoom(1, consts.GameTypeLaiZi)
	defer deleteRoom(room)
	p.RoomID = room.ID
	p.Role = RolePlayer
	getRoomPlayers(room.ID)[p.ID] = true
	room.Players = 1
	if got := ResumeState(p); got != consts.StateWaiting {
		t.Fatalf("等待房间应恢复到房间, got %v", got)
	}

	room.State = consts.RoomStateRunning
	if got := ResumeState(p); got != consts.StateGame {
		t.Fatalf("牌局中应回到牌桌, got %v", got)
	}
}

func TestChatRateLimit(t *testing.T) {
	id := int64(1001)
	// 初始令牌桶允许 3 条突发
	for i := 0; i < 3; i++ {
		if v := chatCheck(id, "msg-"+string(rune('a'+i))); v != chatOK {
			t.Fatalf("第 %d 条正常消息被拦截: %v", i+1, v)
		}
	}
	if v := chatCheck(id, "msg-d"); v != chatThrottled {
		t.Fatalf("超出突发上限的消息未被限流: %v", v)
	}
	// 等待回填后可以继续发言
	time.Sleep(chatInterval + 100*time.Millisecond)
	if v := chatCheck(id, "msg-e"); v != chatOK {
		t.Fatalf("回填后仍然被拦截: %v", v)
	}
}

func TestChatDuplicate(t *testing.T) {
	id := int64(1002)
	msg := "重复刷屏"
	if v := chatCheck(id, msg); v != chatOK {
		t.Fatalf("首条消息被拦截: %v", v)
	}
	if v := chatCheck(id, msg); v != chatDuplicate {
		t.Fatalf("重复消息未被去重: %v", v)
	}
}

// TestChatRingWindow 最近 5 条内的消息视为重复，超出窗口后可以再说
func TestChatRingWindow(t *testing.T) {
	lim := &chatLimit{tokens: chatBurst, last: time.Now()}
	first := hashMessage("第一句")
	lim.push(first)
	for i := 0; i < chatHistory-1; i++ {
		lim.push(hashMessage(string(rune('a' + i))))
	}
	if !lim.seen(first) {
		t.Fatal("还在最近 5 条内却被判为不重复")
	}
	lim.push(hashMessage("newest"))
	if lim.seen(first) {
		t.Fatal("超出 5 条窗口后仍被判为重复")
	}
}

func TestTurnTimerInvalidLimit(t *testing.T) {
	timer := NewTurnTimer(time.Second)
	if timer.Remaining() <= 0 || timer.Remaining() > time.Second {
		t.Fatalf("Remaining 异常: %v", timer.Remaining())
	}
	for i := 0; i < MaxInvalidInputs-1; i++ {
		if timer.Invalid() {
			t.Fatalf("第 %d 次非法输入就熔断了", i+1)
		}
	}
	if !timer.Invalid() {
		t.Fatal("达到上限后未熔断")
	}
	if timer.Expired() {
		t.Fatal("计时器不应在 1 秒内过期")
	}
}

func TestTurnTimerExpired(t *testing.T) {
	timer := NewTurnTimer(10 * time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if !timer.Expired() || timer.Remaining() != 0 {
		t.Fatalf("计时器未过期: expired=%v remain=%v", timer.Expired(), timer.Remaining())
	}
	if _, ok := timer.Ask(&Player{ID: 1}); ok {
		t.Fatal("过期后 Ask 应立即返回 false")
	}
}
