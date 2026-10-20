package Milky_go_sdk

import (
	"fmt"
	"os"
	"testing"
)

// Test Environment Variables List:
// TEST_WS_GATEWAY: WebSocket gateway URL for MilkyGo SDK.
// TEST_REST_GATEWAY: REST API gateway URL for MilkyGo SDK.
// TEST_ACCESS_TOKEN: Access token for authentication with MilkyGo SDK (optional).
// TEST_TARGET_USER_ID: User ID to send private messages to (optional).
// TEST_TARGET_GROUP_ID: Group ID to send group messages to (optional).
// TEST_SEND_FILE_URI: URI of the file to upload to a group (optional).

type TestLogger struct {
}

func (l *TestLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", args...)
}

func (l *TestLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", args...)
}

func (l *TestLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("DEBUG: "+format+"\n", args...)
}

func (l *TestLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("WARN: "+format+"\n", args...)
}

func (l *TestLogger) Info(args ...interface{}) {
	fmt.Println("INFO:", fmt.Sprint(args...))
}

func (l *TestLogger) Error(args ...interface{}) {
	fmt.Println("ERROR:", fmt.Sprint(args...))
}

func (l *TestLogger) Debug(args ...interface{}) {
	fmt.Println("DEBUG:", fmt.Sprint(args...))
}

func (l *TestLogger) Warn(args ...interface{}) {
	fmt.Println("WARN:", fmt.Sprint(args...))
}

func TestMilky(m *testing.T) {
	logger := &TestLogger{}
	fmt.Println("Gateway WS:", os.Getenv("TEST_WS_GATEWAY"))
	session, err := New(os.Getenv("TEST_WS_GATEWAY"), os.Getenv("TEST_REST_GATEWAY"), os.Getenv("TEST_ACCESS_TOKEN"), logger)
	if err != nil {
		m.Fatalf("Failed to create session: %v", err)
	}
	session.AddHandler(func(session2 *Session, m *ReceiveMessage) {
		if m == nil {
			return
		}
		if m.MessageScene == "group" {
			fmt.Printf("Received group message: GroupId %d, MessageSeq %d, ", m.Group.GroupId, m.MessageSeq)
		} else if m.MessageScene == "friend" {
			fmt.Printf("Received friend message: SenderName %s, MessageSeq %d, FriendCategoryID %d, FriendCategoryName %s ", m.Friend.Nickname, m.MessageSeq, m.Friend.Category.CategoryID, m.Friend.Category.CategoryName)
		}
		fmt.Printf("Received message: Sender %d", m.SenderId)
		if m.Segments != nil {
			for _, segment := range m.Segments {
				fmt.Printf(" Segment: %v", segment)
			}
		}
		fmt.Println()
	})
	session.AddHandler(func(session2 *Session, m *FriendRequest) {
		if m == nil {
			return
		}
		fmt.Printf("Received friend request: UserId %d, Comment %s\n", m.InitiatorID, m.Comment)
		// err = session2.AcceptFriendRequest(m.InitiatorUID, false)
		// if err != nil {
		// 	logger.Errorf("Failed to accept friend request: %v", err)
		// 	return
		// }
	})
	session.AddHandler(func(session2 *Session, m *BotOffline) {
		if m == nil {
			return
		}
		fmt.Printf("Bot offline: Reason %s\n", m.Reason)
	})
	session.AddHandler(func(session2 *Session, m *GroupInvitation) {
		if m == nil {
			return
		}
		fmt.Printf("Received group invitation: GroupId %d, InitiatorId %d, RequestId %d\n", m.GroupID, m.InitiatorID, m.InvitationSeq)
		// err = session2.AcceptGroupInvitation(m.GroupID, m.InvitationSeq)
		// if err != nil {
		// 	logger.Errorf("Failed to accept group invite request: %v", err)
		// 	return
		// }
	})
	err = session.Open()
	if err != nil {
		m.Fatalf("Failed to open session: %v", err)
	}
	info, err := session.GetLoginInfo()
	if err != nil {
		m.Fatalf("Failed to get login info: %v", err)
	}
	logger.Infof("Login info: UserId %d, Nickname %s", info.UIN, info.Nickname)
	//forward := OutgoingForwardedMessage{
	//	UserID: info.UIN,
	//	Name:   info.Nickname,
	//	Segments: []IMessageElement{
	//		&TextElement{Text: "This is a message sent by MilkyGo SDK."},
	//	},
	//}
	//forward2 := OutgoingForwardedMessage{
	//	UserID: info.UIN,
	//	Name:   info.Nickname,
	//	Segments: []IMessageElement{
	//		&TextElement{Text: "This is a second message in the same forward."},
	//	},
	//}
	//element := &ForwardElement{
	//	Messages: []OutgoingForwardedMessage{forward, forward2},
	//}
	text := TextElement{
		Text: "Hello, this is a test message from MilkyGo SDK!",
	}
	image := ImageElement{
		URI:     "https://i2.hdslb.com/bfs/archive/7fac120d07a58a936bd877ceeb53f1e6388ee6e7.jpg",
		Summary: "image.jpg",
		SubType: "normal",
	}
	var elements []IMessageElement
	elements = append(elements, &text)
	elements = append(elements, &image)
	friendList, err := session.GetFriendList(true)
	if err != nil {
		m.Fatalf("Failed to get friend list: %v", err)
	}
	logger.Infof("Friend list: %v", friendList)
	groupList, err := session.GetGroupList(true)
	if err != nil {
		m.Fatalf("Failed to get group list: %v", err)
	}
	logger.Infof("Group list: %v", groupList)
	//// get from env
	//targetUserID := os.Getenv("TEST_TARGET_USER_ID")
	//targetUserIDInt, err := strconv.ParseInt(targetUserID, 10, 64)
	//if err != nil {
	//	m.Fatalf("Invalid TEST_TARGET_USER_ID: %v", err)
	//}
	//message, err := session.SendPrivateMessage(targetUserIDInt, &elements)
	//if err != nil {
	//	m.Fatalf("Failed to send private message: %v", err)
	//}
	//targetGroupID := os.Getenv("TEST_TARGET_GROUP_ID")
	//targetGroupIDInt, err := strconv.ParseInt(targetGroupID, 10, 64)
	//if err != nil {
	//	m.Fatalf("Invalid TEST_TARGET_GROUP_ID: %v", err)
	//}
	//err = session.QuitGroup(targetGroupIDInt)
	//if err != nil {
	//	m.Fatalf("Failed to quit group: %v", err)
	//}
	//err = session.SetGroupMemberCard(targetGroupIDInt, targetUserIDInt, "MilkyGo SDK Test Card")
	//if err != nil {
	//	m.Fatalf("Failed to set group member card: %v", err)
	//}
	//file, err := session.UploadGroupFile(targetGroupIDInt, os.Getenv("TEST_SEND_FILE_URI"), "file.txt", "")
	//if err != nil {
	//	m.Fatalf("Failed to upload group file: %v", err)
	//	return
	//}
	//logger.Infof("Uploaded file: %s", file)
	//message, err := session.SendGroupMessage(targetGroupIDInt, &elements)
	//if err != nil {
	//	m.Fatalf("Failed to send group message: %v", err)
	//}
	//logger.Infof("Sent group message: MessageId %d, Time %d", message.MessageSeq, message.Time)
	select {}
}
