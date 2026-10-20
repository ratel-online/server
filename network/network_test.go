package network

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ratel-online/core/protocol"
)

type testClient struct {
	conn net.Conn
	c    protocol.ReadWriteCloser
}

// session 是一次握手的结果：连接与欢迎语。
type session struct {
	tc      *testClient
	welcome string
}

// dial 建立一条连接并完成认证握手。payload 是完整的认证 JSON。
func dial(t *testing.T, payload string) *session {
	t.Helper()
	server, client := net.Pipe()
	go func() { _ = handle(protocol.NewTcpReadWriteCloser(server)) }()

	c := protocol.NewTcpReadWriteCloser(client)
	if err := c.Write(protocol.Packet{Body: []byte(payload)}); err != nil {
		t.Fatalf("发送认证包失败: %v", err)
	}
	s := &session{tc: &testClient{conn: client, c: c}}
	s.welcome = s.tc.read(t)
	return s
}

func authPayloadOf(name string) string {
	return `{"id":1,"name":"` + name + `"}`
}

func (tc *testClient) read(t *testing.T) string {
	t.Helper()
	_ = tc.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	pkt, err := tc.c.Read()
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	return pkt.String()
}

// readOrErr 读一条消息，失败时返回错误（用于验证连接已被断开）。
func (tc *testClient) readOrErr() (string, error) {
	_ = tc.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	pkt, err := tc.c.Read()
	if err != nil {
		return "", err
	}
	return pkt.String(), nil
}

func (tc *testClient) close() {
	_ = tc.conn.Close()
}

// nameTag 从 "Hi 张三#0001, Welcome..." 里取出带短 ID 的展示名。
func nameTag(msg string) string {
	const prefix = "Hi "
	i := strings.Index(msg, prefix)
	if i < 0 {
		return ""
	}
	rest := msg[i+len(prefix):]
	if j := strings.IndexAny(rest, ", \n"); j > 0 {
		return rest[:j]
	}
	return rest
}

// TestHandleNameRules 验证：展示名带 "#短ID"，玩家无法自带 "#" 伪造他人身份。
func TestHandleNameRules(t *testing.T) {
	c1 := dial(t, authPayloadOf("张三"))
	defer c1.tc.close()
	if !strings.Contains(c1.welcome, "张三#") {
		t.Fatalf("名字缺少服务端短 ID: %q", c1.welcome)
	}

	// 同 IP 不同名 -> 独立玩家
	c2 := dial(t, authPayloadOf("李四"))
	defer c2.tc.close()
	if !strings.Contains(c2.welcome, "李四#") {
		t.Fatalf("不同名应允许进入: %q", c2.welcome)
	}

	// 自带 "#ID" 试图伪造他人：'#' 被清除，后缀仍是自己的 ID
	c3 := dial(t, authPayloadOf("张三#0001"))
	defer c3.tc.close()
	if strings.Contains(c3.welcome, "张三#0001") {
		t.Fatalf("玩家可以伪造 ID 后缀: %q", c3.welcome)
	}
	if !strings.Contains(c3.welcome, "张三0001#") {
		t.Fatalf("伪造的名字未被净化: %q", c3.welcome)
	}
}

// TestHandleTakeoverKeepsID 同 IP 同名再次进入时执行顶号：
// 新连接接管原玩家（ID/名字后缀不变），旧连接被断开。
func TestHandleTakeoverKeepsID(t *testing.T) {
	c1 := dial(t, authPayloadOf("顶号测试"))
	defer c1.tc.close()
	tag := nameTag(c1.welcome)
	if tag == "" || !strings.Contains(tag, "#") {
		t.Fatalf("首个连接的名字异常: %q", c1.welcome)
	}

	c2 := dial(t, authPayloadOf("顶号测试"))
	defer c2.tc.close()
	if got := nameTag(c2.welcome); got != tag {
		t.Fatalf("顶号后 ID 未沿用: 旧=%q 新=%q", tag, got)
	}

	// 旧连接应被服务端断开
	if msg, err := c1.tc.readOrErr(); err == nil {
		t.Fatalf("旧连接未被断开，仍收到: %q", msg)
	}

	// 顶号只影响同 IP 同名，其他人不受影响
	c3 := dial(t, authPayloadOf("王五"))
	defer c3.tc.close()
	if !strings.Contains(c3.welcome, "王五#") {
		t.Fatalf("其他玩家受影响: %q", c3.welcome)
	}
}
