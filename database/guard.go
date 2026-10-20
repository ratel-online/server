package database

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/consts"
)

// ============================================================================
// 行为治理：违规计数 -> 禁言 -> 踢房 + IP 封禁
//
// 全部内存态，不依赖数据库：
//   - 3 次违规（骂人/刷屏）-> 禁言 60 秒；
//   - 累计 6 次 -> 踢出房间，并把来源 IP 封禁 30 分钟（重连也进不来）；
//   - 30 分钟没有新违规则计数自动衰减，避免一次口角永久背着。
//
// 计数按 IP 记（"真身"），断了重进换个名字也洗不掉；房主手动禁言按玩家 ID 记。
// ============================================================================

const (
	muteStrikeLimit  = 3
	kickStrikeLimit  = 6
	autoMuteDuration = 60 * time.Second
	ipBanDuration    = 30 * time.Minute
	strikeDecay      = 30 * time.Minute
	chatTableTTL     = 10 * time.Minute
)

var (
	guardMu    sync.Mutex
	strikes    = map[string]int{}         // 违规计数（按 IP）
	strikeAt   = map[string]time.Time{}   // 最近一次违规时间，用于衰减
	autoMutes  = map[string]time.Time{}   // 自动禁言（按 IP）
	manualMute = map[int64]time.Time{}    // 房主手动禁言（按玩家 ID）
	ipBans     = map[string]time.Time{}   // IP 封禁
	ipConns    = map[string]int{}         // 每个 IP 的活跃连接数
	ipAttempts = map[string][]time.Time{} // 每个 IP 的连接尝试时间窗
)

// guardKey 违规计数的主体：优先用 IP（换名字也逃不掉），没有 IP 时退回玩家 ID。
func guardKey(p *Player) string {
	if p == nil {
		return ""
	}
	if p.IP != "" {
		return normalizeIP(p.IP)
	}
	return "id:" + strconv.FormatInt(p.ID, 10)
}

// Strike 记一次违规，返回是否触发禁言/踢房。
func Strike(p *Player, reason string) (muted bool, kicked bool) {
	key := guardKey(p)
	if key == "" {
		return false, false
	}
	now := time.Now()

	guardMu.Lock()
	if last, ok := strikeAt[key]; ok && now.Sub(last) > strikeDecay {
		strikes[key] = 0
	}
	strikes[key]++
	strikeAt[key] = now
	count := strikes[key]
	switch {
	case count >= kickStrikeLimit:
		strikes[key] = 0
		delete(strikeAt, key)
		ipBans[key] = now.Add(ipBanDuration)
		kicked = true
	case count >= muteStrikeLimit:
		autoMutes[key] = now.Add(autoMuteDuration)
		muted = true
	}
	guardMu.Unlock()

	switch {
	case kicked:
		log.Infof("[guard] %s[%d] ip %s kicked & banned %v, reason: %s\n", p.Name, p.ID, key, ipBanDuration, reason)
		punishKick(p)
	case muted:
		log.Infof("[guard] %s[%d] ip %s muted %v, reason: %s\n", p.Name, p.ID, key, autoMuteDuration, reason)
		_ = p.WriteString(fmt.Sprintf("You have been muted for %d seconds due to %s.\n", int(autoMuteDuration.Seconds()), reason))
	}
	return muted, kicked
}

// punishKick 把违规玩家请出房间。等待房间直接踢（并禁止再加入该房间），
// 牌局进行中不动座位（避免打散牌局），只广播 + 依赖禁言与 IP 封禁止损。
func punishKick(p *Player) {
	room := getRoom(p.RoomID)
	if room == nil {
		return
	}
	if room.State == consts.RoomStateWaiting {
		Kicking(room.ID, p.ID)
		Broadcast(room.ID, fmt.Sprintf("%s was kicked for abusive behavior and banned for %d minutes.\n",
			p.Name, int(ipBanDuration.Minutes())))
		return
	}
	Broadcast(p.RoomID, fmt.Sprintf("%s was banned for abusive behavior and will lose the seat after this round.\n", p.Name))
}

// MutedFor 返回玩家剩余的禁言时长（自动禁言与房主禁言取较大值）。
func MutedFor(p *Player) time.Duration {
	if p == nil {
		return 0
	}
	now := time.Now()
	guardMu.Lock()
	defer guardMu.Unlock()

	remain := time.Duration(0)
	if until, ok := autoMutes[guardKey(p)]; ok {
		if d := until.Sub(now); d > remain {
			remain = d
		}
	}
	if until, ok := manualMute[p.ID]; ok {
		if d := until.Sub(now); d > remain {
			remain = d
		}
	}
	return remain
}

// MutePlayer 房主手动禁言指定玩家。
func MutePlayer(id int64, d time.Duration) {
	if d <= 0 {
		d = 5 * time.Minute
	}
	guardMu.Lock()
	manualMute[id] = time.Now().Add(d)
	guardMu.Unlock()
}

// UnmutePlayer 解除房主手动禁言，返回此前是否处于禁言状态。
func UnmutePlayer(id int64) bool {
	guardMu.Lock()
	_, existed := manualMute[id]
	delete(manualMute, id)
	guardMu.Unlock()
	return existed
}

// IsIPBanned 判断 IP 是否在封禁期内，并返回剩余时间。
func IsIPBanned(ip string) (bool, time.Duration) {
	ip = normalizeIP(ip)
	if ip == "" {
		return false, 0
	}
	guardMu.Lock()
	defer guardMu.Unlock()
	until, ok := ipBans[ip]
	if !ok {
		return false, 0
	}
	if time.Now().After(until) {
		delete(ipBans, ip)
		return false, 0
	}
	return true, time.Until(until)
}

// AllowConnection 连接准入：同一 IP 的连接数上限 + 连接速率限制。
// takeover 为顶号（同 IP 同名重连），只做速率限制，不占新的名额判断，
// 否则名额打满后本人掉线就再也回不来。
func AllowConnection(ip string, takeover bool) error {
	ip = normalizeIP(ip)
	parsed := net.ParseIP(ip)
	// 拿不到真实 IP（unix socket、测试管道等）时不限制
	if parsed == nil {
		return nil
	}
	// 回环地址只可能是本机（本地联调、自动化测试），不做准入限制，
	// 否则同机起几个测试客户端就会撞上限流。
	if parsed.IsLoopback() {
		return nil
	}
	now := time.Now()

	guardMu.Lock()
	defer guardMu.Unlock()

	kept := ipAttempts[ip][:0]
	for _, t := range ipAttempts[ip] {
		if now.Sub(t) <= consts.ConnRateWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) >= consts.ConnRateLimit {
		ipAttempts[ip] = kept
		return consts.ErrorsConnectTooFast
	}
	ipAttempts[ip] = append(kept, now)

	if !takeover && ipConns[ip] >= consts.MaxConnsPerIP {
		return consts.ErrorsTooManyConns
	}
	ipConns[ip]++
	return nil
}

// ReleaseConnection 连接关闭时释放名额。
func ReleaseConnection(ip string) {
	ip = normalizeIP(ip)
	if ip == "" {
		return
	}
	guardMu.Lock()
	defer guardMu.Unlock()
	if n, ok := ipConns[ip]; ok {
		if n <= 1 {
			delete(ipConns, ip)
		} else {
			ipConns[ip] = n - 1
		}
	}
}

// Maintenance 由房间清理循环每分钟调用一次，回收各类过期数据，避免内存只增不减。
func Maintenance() {
	now := time.Now()
	guardMu.Lock()
	for k, until := range autoMutes {
		if now.After(until) {
			delete(autoMutes, k)
		}
	}
	for k, until := range manualMute {
		if now.After(until) {
			delete(manualMute, k)
		}
	}
	for k, until := range ipBans {
		if now.After(until) {
			delete(ipBans, k)
		}
	}
	for k, at := range strikeAt {
		if now.Sub(at) > strikeDecay {
			delete(strikeAt, k)
			delete(strikes, k)
		}
	}
	for ip, list := range ipAttempts {
		kept := list[:0]
		for _, t := range list {
			if now.Sub(t) <= consts.ConnRateWindow {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(ipAttempts, ip)
		} else {
			ipAttempts[ip] = kept
		}
	}
	for ip, n := range ipConns {
		if n <= 0 {
			delete(ipConns, ip)
		}
	}
	guardMu.Unlock()

	chatMu.Lock()
	for id, lim := range chatTable {
		if now.Sub(lim.last) > chatTableTTL {
			delete(chatTable, id)
		}
	}
	chatMu.Unlock()

	pruneNameIndex()
}

// pruneNameIndex 回收长时间离线、又不在牌局里的玩家所占的名字，
// 避免名字索引只增不减（名字在断线时会保留，方便本人重连接管）。
func pruneNameIndex() {
	nameMu.Lock()
	defer nameMu.Unlock()
	for key, owner := range ipNameIndex {
		if owner == nil {
			delete(ipNameIndex, key)
			continue
		}
		if owner.IsOnline() {
			continue
		}
		if room := getRoom(owner.RoomID); room != nil && room.State == consts.RoomStateRunning {
			continue // 座位还留着，名字也要留着才能顶号
		}
		if owner.OfflineFor() > consts.WaitingIdleTimeout {
			delete(ipNameIndex, key)
		}
	}
}
