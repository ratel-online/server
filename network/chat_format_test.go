package network_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ratel-online/server/network"
)

// 聊天行的格式回归：每条聊天必须自己占一行。
// 之前 sanitizeChat 用 TrimSpace 把结尾的 "\n" 一起裁掉了，
// 结果别人看到的聊天全挤在一行，发送者那边又顶出一片空行。
func TestChatLineFormat(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	srv := network.NewTcpServer(addr)
	go func() { _ = srv.Serve() }()
	time.Sleep(200 * time.Millisecond)

	owner := dial(t, addr, "owner")
	other := dial(t, addr, "other")

	waitFor(t, owner, "1.Join")
	owner.send("2")
	waitFor(t, owner, "Please select game type")
	owner.send("1") // 斗地主，等待房间即可聊天
	created := waitFor(t, owner, "Create room successful")
	roomID := "1"
	if idx := strings.LastIndex(created, "id : "); idx >= 0 {
		roomID = strings.TrimSpace(created[idx+len("id : "):])
	}

	waitFor(t, other, "1.Join")
	other.send("1")
	waitFor(t, other, "ID")
	other.send(roomID)
	time.Sleep(300 * time.Millisecond)

	owner.send("hello 世界")
	time.Sleep(150 * time.Millisecond)
	owner.send("second line")
	time.Sleep(150 * time.Millisecond)
	// 多行输入必须被压成一行，不能在别人屏幕上顶出空行
	owner.send("line1\n\n\nline2")
	time.Sleep(300 * time.Millisecond)

	chatLines := collectChat(t, other)
	if len(chatLines) != 3 {
		t.Fatalf("应收到 3 条聊天，实际 %d 条: %q", len(chatLines), chatLines)
	}
	for _, line := range chatLines {
		if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
			t.Fatalf("聊天行必须以且仅以一个换行结尾: %q", line)
		}
	}
	// 短 ID 由玩家 ID 生成，用例之间会递增，只校验格式
	if !regexp.MustCompile(`^>> owner#[0-9A-Z]{4} \[owner\] say: hello 世界\n$`).MatchString(chatLines[0]) {
		t.Fatalf("聊天行格式不对: %q", chatLines[0])
	}
	if !strings.Contains(chatLines[2], "line1 line2") {
		t.Fatalf("多行输入没有被压成一行: %q", chatLines[2])
	}
}

// collectChat 收一段时间，返回所有聊天行（跳过进房之类的系统广播）。
func collectChat(t *testing.T, c *testClient) []string {
	t.Helper()
	out := make([]string, 0, 4)
	for {
		text, err := c.read(400 * time.Millisecond)
		if err != nil {
			return out
		}
		if strings.HasPrefix(text, ">> ") && strings.Contains(text, " say: ") {
			out = append(out, text)
		}
	}
}
