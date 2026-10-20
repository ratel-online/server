package state

import (
	"strconv"
	"testing"

	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/network"
	"github.com/ratel-online/core/protocol"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

// fakeConn 只提供 IP 的假连接，让每个测试玩家有独立的来源 IP。
type fakeConn struct{ ip string }

func (f fakeConn) Read() (*protocol.Packet, error) { select {} }
func (f fakeConn) Write(protocol.Packet) error     { return nil }
func (f fakeConn) Close() error                    { return nil }
func (f fakeConn) IP() string                      { return f.ip }

func newTestPlayer(t *testing.T, name, ip string) (*database.Player, *database.ConnState) {
	t.Helper()
	p, cs, _, err := database.Connected(network.Wrapper(fakeConn{ip: ip}), &model.AuthInfo{Name: name})
	if err != nil {
		t.Fatalf("创建玩家 %s 失败: %v", name, err)
	}
	return p, cs
}

// TestOwnerMuteCommand 房主可以用 mute/unmute 管人，非房主不行
func TestOwnerMuteCommand(t *testing.T) {
	owner, ownerCS := newTestPlayer(t, "房主", "10.1.0.1")
	defer owner.OfflineConn(ownerCS)
	other, otherCS := newTestPlayer(t, "路人", "10.1.0.2")
	defer other.OfflineConn(otherCS)

	room := database.CreateRoom(owner.ID, consts.GameTypeClassic)
	defer database.LeaveRoom(room.ID, owner.ID)
	if err := database.JoinRoom(room.ID, owner.ID); err != nil {
		t.Fatalf("房主进房失败: %v", err)
	}
	if err := database.JoinRoom(room.ID, other.ID); err != nil {
		t.Fatalf("玩家进房失败: %v", err)
	}

	s := &waiting{}
	otherID := strconv.FormatInt(other.ID, 10)

	handled, ok, err := s.ownerCommand(owner, room, []string{"mute", otherID, "2"})
	if !handled || ok || err != nil {
		t.Fatalf("mute 指令处理异常: handled=%v ok=%v err=%v", handled, ok, err)
	}
	if database.MutedFor(other) <= 0 {
		t.Fatal("房主禁言未生效")
	}
	if database.BroadcastChat(other, "hello") {
		t.Fatal("被禁言的玩家仍然能发言")
	}

	// 解除禁言
	if _, _, err := s.ownerCommand(owner, room, []string{"unmute", otherID}); err != nil {
		t.Fatalf("unmute 处理异常: %v", err)
	}
	if database.MutedFor(other) > 0 {
		t.Fatal("解除禁言后仍然被禁言")
	}

	// 非房主不能禁言别人
	ownerID := strconv.FormatInt(owner.ID, 10)
	if handled, _, _ := s.ownerCommand(other, room, []string{"mute", ownerID}); handled {
		t.Fatal("非房主竟然能执行 mute")
	}
	// 房主也不能禁言不在房间里的玩家（指令被消费，但不生效）
	if handled, _, _ := s.ownerCommand(owner, room, []string{"mute", "999999"}); !handled {
		t.Fatal("对不在房间的玩家 mute 未被处理")
	}
}

// TestOwnerKickCommand 房主踢人，被踢的人不能再进同一个房间
func TestOwnerKickCommand(t *testing.T) {
	owner, ownerCS := newTestPlayer(t, "房主二", "10.1.1.1")
	defer owner.OfflineConn(ownerCS)
	other, otherCS := newTestPlayer(t, "路人二", "10.1.1.2")
	defer other.OfflineConn(otherCS)

	room := database.CreateRoom(owner.ID, consts.GameTypeClassic)
	defer database.LeaveRoom(room.ID, owner.ID)
	_ = database.JoinRoom(room.ID, owner.ID)
	_ = database.JoinRoom(room.ID, other.ID)

	s := &waiting{}
	if handled, _, _ := s.ownerCommand(owner, room, []string{"k", strconv.FormatInt(other.ID, 10)}); !handled {
		t.Fatal("踢人指令未被处理")
	}
	if other.RoomID != 0 {
		t.Fatalf("被踢玩家仍在房间里: %d", other.RoomID)
	}
	if err := database.JoinRoom(room.ID, other.ID); err != consts.ErrorsJoinFailForKicked {
		t.Fatalf("被踢玩家还能回房间, got %v", err)
	}
}

// TestStartRequiresEnoughPlayers 人不够时不能开局
func TestStartRequiresEnoughPlayers(t *testing.T) {
	owner, ownerCS := newTestPlayer(t, "房主三", "10.1.2.1")
	defer owner.OfflineConn(ownerCS)

	room := database.CreateRoom(owner.ID, consts.GameTypeClassic)
	defer database.LeaveRoom(room.ID, owner.ID)
	_ = database.JoinRoom(room.ID, owner.ID)

	s := &waiting{}
	handled, ok, err := s.ownerCommand(owner, room, []string{"start"})
	if !handled || ok || err != nil {
		t.Fatalf("单人开局应被拒绝: handled=%v ok=%v err=%v", handled, ok, err)
	}
	if room.State != consts.RoomStateWaiting {
		t.Fatal("单人竟然开局了")
	}
}
