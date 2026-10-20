package database

import (
	"hash/fnv"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/network"
	coreStrings "github.com/ratel-online/core/util/strings"
	"github.com/ratel-online/server/consts"
)

// ============================================================================
// 名字治理：净化 + 同 IP 同名排他 + 服务端短 ID 后缀
//
// 玩家上报的名字不可信（可随便填别人的名字），所以这里做三件事：
//  1. 净化：全角转半角、去控制符/零宽字符/空白/'#'，限长；
//  2. 排他：同一个 IP 下不允许同名在线，ipNameIndex 维护 "IP+名字 -> 玩家 ID"；
//  3. 标记：展示名统一由服务端拼成 "名字#短ID"，玩家无法伪造 '#'。
// 不同 IP 之间允许同名（重名靠 #ID 区分，不做名字后缀编号）。
// 全程只用内存 map，不依赖任何数据库。
// ============================================================================

const (
	maxNameRunes = 12
	defaultName  = "player"
)

var (
	nameMu      sync.RWMutex
	ipNameIndex = map[string]*Player{} // "规范化IP\x00小写名字" -> 在线玩家
)

// normalizeIP 去掉端口，只保留主机部分，保证同一来源的不同连接能对上。
func normalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

// nameKey 生成同一 IP + 同名 的排他键。
func nameKey(ip, base string) string {
	return normalizeIP(ip) + "\x00" + strings.ToLower(base)
}

// sanitizeName 清理客户端上报的名字。
func sanitizeName(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		// 全角 ASCII -> 半角
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0
		}
		switch {
		case r == '#': // '#' 由服务端独占，禁止玩家自己带
			continue
		case r == 0x3000: // 全角空格
			continue
		case unicode.IsSpace(r), unicode.IsControl(r):
			continue
		case r >= 0x200B && r <= 0x200F: // 零宽字符
			continue
		case r == 0xFEFF, r == 0x2028, r == 0x2029:
			continue
		}
		b.WriteRune(r)
		if b.Len() >= maxNameRunes*4 {
			break
		}
	}
	name := strings.TrimSpace(b.String())
	if name == "" {
		name = defaultName
	}
	runes := []rune(name)
	if len(runes) > maxNameRunes {
		name = string(runes[:maxNameRunes])
	}
	return name
}

// shortID 把玩家 ID 转成 4 位短标识，用于名字后缀。
func shortID(id int64) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	if id < 0 {
		id = -id
	}
	b := [4]byte{'0', '0', '0', '0'}
	for i := 3; i >= 0 && id > 0; i-- {
		b[i] = alphabet[id%36]
		id /= 36
	}
	return string(b[:])
}

// assignName 为新玩家登记名字：展示名为 "基础名#短ID"。
// 同一 IP 下已有同名在线玩家时返回错误（是否顶号由 claimIdentity 决定）。
func assignName(p *Player, raw string) error {
	// 先过一遍脏词替换，再净化，保证名字里的 '#' 只可能由服务端追加
	base := sanitizeName(coreStrings.Desensitize(raw))
	return registerName(p, base, nameKey(p.IP, base))
}

// registerName 登记名字占用并生成展示名。
func registerName(p *Player, base, key string) error {
	nameMu.Lock()
	defer nameMu.Unlock()

	if owner, exists := ipNameIndex[key]; exists && owner != p && owner.IsOnline() {
		return consts.ErrorsNameInUse
	}
	ipNameIndex[key] = p
	p.baseName = base
	p.nameKey = key
	p.Name = base + "#" + shortID(p.ID)
	return nil
}

// lookupSameIPName 查找同 IP 同名的在线玩家（即顶号目标）。
func lookupSameIPName(ip, base string) *Player {
	nameMu.Lock()
	defer nameMu.Unlock()
	owner := ipNameIndex[nameKey(ip, base)]
	if owner == nil || !owner.IsOnline() {
		return nil
	}
	return owner
}

// claimIdentity 为本次连接确定身份：
//   - 同 IP + 同名且原玩家仍在线 -> 顶号，沿用原玩家的 ID、名字、房间与牌局进度，只替换连接；
//   - 否则 -> 新建玩家并登记名字。
//
// 返回的 tookOver 表示本次是顶号。
func claimIdentity(conn *network.Conn, raw string) (p *Player, cs *ConnState, tookOver bool, err error) {
	base := sanitizeName(coreStrings.Desensitize(raw))
	ip := normalizeIP(conn.IP())
	key := nameKey(ip, base)

	if owner := lookupSameIPName(ip, base); owner != nil && owner.ID != conn.ID() {
		snap := owner.snapshot()
		owner.invalidate() // 旧连接、旧状态机一并作废，槽位 ID 留给新连接用

		p = &Player{
			ID:       snap.ID,
			IP:       ip,
			Name:     snap.Name,
			Mode:     snap.Mode,
			Type:     snap.Type,
			Amount:   snap.Amount,
			RoomID:   snap.RoomID,
			Role:     snap.Role,
			baseName: snap.baseName,
			nameKey:  snap.nameKey,
		}
		if p.baseName == "" {
			p.baseName = base
			p.Name = base + "#" + shortID(p.ID)
		}
		cs = p.BindConn(conn)
		nameMu.Lock()
		ipNameIndex[key] = p
		nameMu.Unlock()
		log.Infof("[identity] %s[%d] taken over by a new connection from %s\n", p.Name, p.ID, ip)
		return p, cs, true, nil
	}

	p = &Player{ID: conn.ID(), IP: ip, Amount: 2000}
	cs = p.BindConn(conn)
	if err = registerName(p, base, key); err != nil {
		cs.close()
		return nil, nil, false, err
	}
	return p, cs, false, nil
}

// releaseName 玩家离线后释放名字占用，避免名字被永久锁死。
func releaseName(p *Player) {
	if p == nil || p.nameKey == "" {
		return
	}
	nameMu.Lock()
	defer nameMu.Unlock()
	// 可能已经被新连接接管，只有仍属于自己时才删
	if owner, ok := ipNameIndex[p.nameKey]; ok && owner == p {
		delete(ipNameIndex, p.nameKey)
	}
}

// BaseName 返回不含 "#短ID" 后缀的名字，用于日志等场景。
func (p *Player) BaseName() string {
	if p.baseName != "" {
		return p.baseName
	}
	return p.Name
}

// ============================================================================
// 聊天限流：令牌桶 + 重复消息去重 + 违规禁言
// ============================================================================

const (
	chatBurst     = 3.0             // 允许的突发条数
	chatInterval  = 2 * time.Second // 每 2 秒回 1 条
	chatStrikes   = 5               // 连续被拦截多少次后禁言
	chatMuteTime  = 60 * time.Second
	chatDupWindow = 2 // 同一条消息连续出现超过该次数就丢弃
)

type chatLimit struct {
	tokens    float64
	last      time.Time
	strikes   int
	repeat    int
	lastHash  uint64
	muteUntil time.Time
}

var (
	chatMu    sync.Mutex
	chatTable = map[int64]*chatLimit{}
)

func hashMessage(msg string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(msg))
	return h.Sum64()
}

// allowChat 判断该玩家当前是否可以发言（内存态，无需数据库）。
func allowChat(id int64, msg string) bool {
	now := time.Now()
	chatMu.Lock()
	defer chatMu.Unlock()

	lim, ok := chatTable[id]
	if !ok {
		lim = &chatLimit{tokens: chatBurst, last: now}
		chatTable[id] = lim
	}
	if now.Before(lim.muteUntil) {
		return false
	}
	lim.tokens += now.Sub(lim.last).Seconds() / chatInterval.Seconds()
	if lim.tokens > chatBurst {
		lim.tokens = chatBurst
	}
	lim.last = now

	h := hashMessage(msg)
	if h == lim.lastHash {
		lim.repeat++
	} else {
		lim.lastHash = h
		lim.repeat = 0
	}
	if lim.repeat > chatDupWindow {
		return false
	}

	if lim.tokens < 1 {
		lim.strikes++
		if lim.strikes >= chatStrikes {
			lim.muteUntil = now.Add(chatMuteTime)
			lim.strikes = 0
			log.Infof("[chat] player %d muted for %v (flood)\n", id, chatMuteTime)
		}
		return false
	}
	lim.tokens--
	return true
}

// ============================================================================
// 回合计时器：绝对截止时间 + 连续非法输入熔断
//
// 历史实现用 time.Now().Unix() 做秒级差值再累减 timeout，
// 熊只要每 0.5 秒发一条垃圾输入，差值恒为 0，timeout 永远减不完 -> 无限拖时间。
// 这里改成绝对 deadline，任何输入消耗的都是真实时间。
// ============================================================================

// MaxInvalidInputs 连续多少次非法输入后直接按超时处理。
const MaxInvalidInputs = 3

type TurnTimer struct {
	deadline time.Time
	timeout  time.Duration
	invalid  int
}

func NewTurnTimer(d time.Duration) *TurnTimer {
	return &TurnTimer{deadline: time.Now().Add(d), timeout: d}
}

// Reset 重置计时（用于强制答案重试等场景）。
func (t *TurnTimer) Reset(d time.Duration) {
	t.timeout = d
	t.deadline = time.Now().Add(d)
	t.invalid = 0
}

func (t *TurnTimer) Remaining() time.Duration {
	remain := time.Until(t.deadline)
	if remain < 0 {
		return 0
	}
	return remain
}

func (t *TurnTimer) Expired() bool {
	return !time.Now().Before(t.deadline)
}

// Ask 在剩余时间内等待玩家输入，ok=false 表示超时/断线。
func (t *TurnTimer) Ask(p *Player) (string, bool) {
	ans, err := t.AskErr(p)
	if err != nil {
		return "", false
	}
	return ans, true
}

// AskErr 同 Ask，但保留具体错误（用于区分超时与断线）。
func (t *TurnTimer) AskErr(p *Player) (string, error) {
	remain := t.Remaining()
	if remain <= 0 {
		return "", consts.ErrorsTimeout
	}
	return p.AskForString(remain)
}

// Invalid 记录一次非法输入，返回 true 表示已达熔断阈值，应按超时处理。
func (t *TurnTimer) Invalid() bool {
	t.invalid++
	if t.invalid >= MaxInvalidInputs {
		t.invalid = 0
		return true
	}
	return false
}
