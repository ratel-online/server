package network_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ratel-online/server/network"
)

type testClient struct {
	name         string
	conn         net.Conn
	mu           sync.Mutex
	log          strings.Builder
	chatted      bool
	chattedClaim bool
}

func dial(t *testing.T, addr, name string) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &testClient{name: name, conn: conn}
	info, _ := json.Marshal(map[string]interface{}{"name": name})
	c.send(string(info))
	return c
}

func (c *testClient) send(msg string) {
	body := []byte(msg)
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(buf, uint32(len(body)))
	copy(buf[4:], body)
	_, _ = c.conn.Write(buf)
}

func (c *testClient) read(timeout time.Duration) (string, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	head := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, head); err != nil {
		return "", err
	}
	size := binary.BigEndian.Uint32(head)
	body := make([]byte, size)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return "", err
	}
	text := string(body)
	c.mu.Lock()
	c.log.WriteString(text)
	c.mu.Unlock()
	return text, nil
}

// bot 自动应答：出牌类提示出第一张；aggressive 为 true 时优先胡 > 杠 > 碰 > 吃，否则一律过
func (c *testClient) bot(done chan<- string, aggressive bool) {
	for {
		text, err := c.read(20 * time.Second)
		if err != nil {
			done <- "read error: " + err.Error()
			return
		}
		switch {
		case strings.Contains(text, "牌局结束"):
			done <- "ok"
			return
		case strings.Contains(text, "【轮到你】"):
			// 第一次轮到自己时先说句话，验证牌局中聊天不会被当成错误输入
			if !c.chatted {
				c.chatted = true
				c.send("大家好，这局我胡了")
				continue
			}
			if strings.Contains(text, "可胡：") {
				c.send("H")
				continue
			}
			if tile := firstTile(text); tile != "" {
				c.send(tile)
			}
		case strings.Contains(text, "【可操作】"), strings.Contains(text, "【抢杠】"):
			// 第一次收到询问时先聊一句，验证聊天不会顶掉这次操作机会
			if !c.chattedClaim {
				c.chattedClaim = true
				c.send("先想想，别急")
				continue
			}
			if aggressive {
				switch {
				case strings.Contains(text, "H. 胡牌"), strings.Contains(text, "H. 抢杠胡"):
					c.send("H")
				case strings.Contains(text, "G. 杠"):
					c.send("G")
				case strings.Contains(text, "P. 碰"):
					c.send("P")
				case strings.Contains(text, "C1."):
					c.send("C1")
				default:
					c.send("0")
				}
				continue
			}
			c.send("0")
		}
	}
}

func firstTile(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		if !strings.HasPrefix(line, "手牌(") {
			continue
		}
		idx := strings.Index(line, "：")
		if idx < 0 {
			continue
		}
		text := strings.ReplaceAll(line[idx+len("："):], "|", " ")
		fields := strings.Fields(text)
		if len(fields) > 0 {
			return strings.TrimPrefix(fields[0], "*")
		}
	}
	return ""
}

// freePort 取一个空闲端口，避免测试之间互相占用
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func playMahjong(t *testing.T, players int, aggressive bool) string {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	srv := network.NewTcpServer(addr)
	go func() { _ = srv.Serve() }()
	time.Sleep(200 * time.Millisecond)

	clients := make([]*testClient, 0, players)
	for i := 0; i < players; i++ {
		clients = append(clients, dial(t, addr, fmt.Sprintf("bot%d", i+1)))
	}

	// 建房
	owner := clients[0]
	waitFor(t, owner, "1.Join")
	owner.send("2")
	waitFor(t, owner, "Please select game type")
	owner.send("6")
	created := waitFor(t, owner, "Create room successful")
	roomID := "1"
	if idx := strings.LastIndex(created, "id : "); idx >= 0 {
		roomID = strings.TrimSpace(created[idx+len("id : "):])
	}

	// 其他人加入同一房间
	for _, c := range clients[1:] {
		waitFor(t, c, "1.Join")
		c.send("1")
		waitFor(t, c, "ID")
		c.send(roomID)
	}
	time.Sleep(300 * time.Millisecond)

	// 开始游戏
	owner.send("s")

	results := make(chan string, players)
	for _, c := range clients {
		ch := make(chan string, 1)
		go func(c *testClient, ch chan string) { c.bot(ch, aggressive) }(c, ch)
		go func(ch chan string) { results <- <-ch }(ch)
	}
	for i := 0; i < players; i++ {
		select {
		case r := <-results:
			if r != "ok" {
				t.Fatalf("player %d 结束异常: %s", i, r)
			}
		case <-time.After(90 * time.Second):
			t.Fatalf("牌局超时未结束（%d 人局），可能死锁", players)
		}
	}

	for _, c := range clients {
		_ = c.conn.Close()
	}
	c := clients[0]
	c.mu.Lock()
	out := c.log.String()
	c.mu.Unlock()
	if testing.Verbose() {
		t.Logf("=== %s 收到的牌局输出 ===\n%s", c.name, out)
	}
	return out
}

func waitFor(t *testing.T, c *testClient, want string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		text, err := c.read(5 * time.Second)
		if err != nil {
			t.Fatalf("%s 等待 %q 失败: %v", c.name, want, err)
		}
		if strings.Contains(text, want) {
			return text
		}
	}
	t.Fatalf("%s 等待 %q 超时", c.name, want)
	return ""
}

func TestMahjongThreePlayers(t *testing.T) {
	out := playMahjong(t, 3, false)
	if strings.Contains(out, "【流局】") && !strings.Contains(out, "大众麻将") {
		t.Fatal("牌局输出异常")
	}
	// 牌局中打字应当作为聊天广播，而不是提示"无法识别"
	if !strings.Contains(out, "大家好，这局我胡了") {
		t.Fatalf("牌局中的聊天没有广播出去：\n%s", out)
	}
	if strings.Contains(out, "无法识别") {
		t.Fatalf("聊天被当成非法输入拦下了：\n%s", out)
	}
}

func TestMahjongFourPlayers(t *testing.T) {
	playMahjong(t, 4, false)
}

// 见牌就吃碰杠、能胡就胡，覆盖副露与胡牌结算路径
func TestMahjongClaims(t *testing.T) {
	out := playMahjong(t, 4, true)
	if !strings.Contains(out, "吃") && !strings.Contains(out, "碰") && !strings.Contains(out, "杠") {
		t.Logf("本局未触发副露，输出：\n%s", out)
	}
	if !strings.Contains(out, "【胡牌】") && !strings.Contains(out, "【流局】") {
		t.Fatalf("牌局没有正常结算：\n%s", out)
	}
	// 胡牌时必须按「1 番 = 100 积分」真正结算积分
	if strings.Contains(out, "【胡牌】") {
		if !strings.Contains(out, "1 番 100 积分") {
			t.Fatalf("结算缺少积分换算说明：\n%s", out)
		}
		if !strings.Contains(out, "积分：") || !strings.Contains(out, "→") {
			t.Fatalf("结算缺少积分变动：\n%s", out)
		}
	}
}
