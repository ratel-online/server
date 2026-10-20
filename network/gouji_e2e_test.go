package network_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ratel-online/server/network"
)

// goujiCardValue 牌面点数值，与够级大小顺序一致
func goujiCardValue(token string) int {
	switch strings.ToLower(strings.ReplaceAll(token, "10", "0")) {
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	case "9":
		return 9
	case "0", "t":
		return 10
	case "j":
		return 11
	case "q":
		return 12
	case "k":
		return 13
	case "a", "1":
		return 14
	case "2":
		return 15
	case "s":
		return 16
	case "x":
		return 17
	}
	return -1
}

func goujiAlias(token string) string {
	return strings.ToLower(strings.ReplaceAll(token, "10", "0"))
}

// goujiHandTokens 从 "Timeout: %ds, pokers: ..." 提示行解析手牌（降序显示）
func goujiHandTokens(prompt string) []string {
	for _, line := range strings.Split(prompt, "\n") {
		if !strings.HasPrefix(line, "Timeout:") {
			continue
		}
		idx := strings.Index(line, "pokers: ")
		if idx < 0 {
			return nil
		}
		return strings.Fields(line[idx+len("pokers: "):])
	}
	return nil
}

// goujiLastTokens 从 "Last: %s played ..." 提示行解析上家出的牌
func goujiLastTokens(prompt string) []string {
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, "Last: ") {
			idx := strings.Index(line, "played ")
			if idx < 0 {
				return nil
			}
			return strings.Fields(line[idx+len("played "):])
		}
	}
	return nil
}

// goujiBeatSingle 上家出的是单张时，找手牌里能压住的最小单张；找不到返回空
func goujiBeatSingle(hand, last []string) string {
	if len(last) != 1 {
		return ""
	}
	lastVal := goujiCardValue(last[0])
	if lastVal < 0 {
		return ""
	}
	best := ""
	bestVal := 99
	for _, token := range hand {
		v := goujiCardValue(token)
		if v > lastVal && v < bestVal {
			best, bestVal = goujiAlias(token), v
		}
	}
	return best
}

// goujiBot 自动打牌：轮到自己（无 Last 行）就出最小的单张；
// 上家出单张且手里有更大的单张就压，否则过牌，直到有人出完触发结算。
func (c *testClient) goujiBot(done chan<- string) {
	for {
		// 读超时给足余量：整包测试并发跑时机器负载高，30s 曾出现偶发超时
		text, err := c.read(120 * time.Second)
		if err != nil {
			done <- "read error: " + err.Error()
			return
		}
		switch {
		case strings.Contains(text, "Game over! Results:"):
			done <- "ok"
			return
		case strings.Contains(text, "Timeout:"):
			hand := goujiHandTokens(text)
			if strings.Contains(text, "Last: ") {
				if beat := goujiBeatSingle(hand, goujiLastTokens(text)); beat != "" {
					c.send(beat)
				} else {
					c.send("p")
				}
			} else if len(hand) > 0 {
				// 手牌降序显示，最后一张最小
				c.send(goujiAlias(hand[len(hand)-1]))
			} else {
				done <- "empty hand prompt"
				return
			}
		}
	}
}

// TestGoujiSixPlayers 6 人完整跑一局够级，验证发牌、轮转、结算与积分不卡死。
func TestGoujiSixPlayers(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	srv := network.NewTcpServer(addr)
	go func() { _ = srv.Serve() }()
	time.Sleep(200 * time.Millisecond)

	const players = 6
	clients := make([]*testClient, 0, players)
	for i := 0; i < players; i++ {
		clients = append(clients, dial(t, addr, fmt.Sprintf("gjbot%d", i+1)))
	}

	owner := clients[0]
	waitFor(t, owner, "1.Join")
	owner.send("2")
	waitFor(t, owner, "Please select game type")
	owner.send("10")
	created := waitFor(t, owner, "Create room successful")
	roomID := "1"
	if idx := strings.LastIndex(created, "id : "); idx >= 0 {
		roomID = strings.TrimSpace(created[idx+len("id : "):])
	}

	for _, c := range clients[1:] {
		waitFor(t, c, "1.Join")
		c.send("1")
		waitFor(t, c, "ID")
		c.send(roomID)
	}
	time.Sleep(300 * time.Millisecond)

	owner.send("s")
	// 房主先确认进入牌局（Game starting）
	waitFor(t, owner, "Game starting")

	results := make(chan string, players)
	for _, c := range clients {
		ch := make(chan string, 1)
		go c.goujiBot(ch)
		go func(ch chan string) { results <- <-ch }(ch)
	}
	for i := 0; i < players; i++ {
		select {
		case r := <-results:
			if r != "ok" {
				for j, c := range clients {
					c.mu.Lock()
					t.Logf("=== client %d (%s) log ===\n%s", j, c.name, c.log.String())
					c.mu.Unlock()
				}
				t.Fatalf("player %d 结束异常: %s", i, r)
			}
		case <-time.After(60 * time.Second):
			for j, c := range clients {
				c.mu.Lock()
				t.Logf("=== client %d (%s) log ===\n%s", j, c.name, c.log.String())
				c.mu.Unlock()
			}
			t.Fatalf("牌局超时未结束，可能死锁")
		}
	}

	out := func() string {
		c := clients[0]
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.log.String()
	}()
	if !strings.Contains(out, "Game over! Results:") {
		t.Fatalf("没有看到结算输出：\n%s", out)
	}
	// 结算必须包含 6 个名次与积分变动
	for _, rank := range []string{"头科", "二科", "三科", "四科", "五落", "大落"} {
		if !strings.Contains(out, rank) {
			t.Fatalf("结算缺少名次 %s：\n%s", rank, out)
		}
	}
	if testing.Verbose() {
		t.Logf("=== 够级结算输出 ===\n%s", out[strings.Index(out, "Game over!"):])
	}

	for _, c := range clients {
		_ = c.conn.Close()
	}
}
