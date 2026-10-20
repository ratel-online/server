package Milky_go_sdk

// See events.go
// Following are all the event types.
// Event type values are used to match the events returned
const (
	botOfflineEventType                = "bot_offline"
	messageReceiveEventType            = "message_receive"
	messageRecallEventType             = "message_recall"
	friendRequestEventType             = "friend_request"
	friendNudgeEventType               = "friend_nudge"
	friendFileUploadEventType          = "friend_file_upload"
	groupNudgeEventType                = "group_nudge"
	groupMessageReactionEventType      = "group_message_reaction"
	groupAdminChangeEventType          = "group_admin_change"
	groupEssenceMessageChangeEventType = "group_essence_message_change"
	groupNameChangeEventType           = "group_name_change"
	groupFileUploadEventType           = "group_file_upload"
	groupMuteEventType                 = "group_mute"
	groupWholeMuteEventType            = "group_whole_mute"
	groupMemberIncreaseEventType       = "group_member_increase"
	groupMemberDecreaseEventType       = "group_member_decrease"
	groupJoinRequestEventType          = "group_join_request"
	groupInvitedJoinRequestEventType   = "group_invited_join_request"
	groupInvitationEventType           = "group_invitation"
)

func handlerForInterface(handler interface{}) EventHandler {
	switch v := handler.(type) {
	case func(*Session, interface{}):
		return interfaceEventHandler(v)
	case func(*Session, *ReceiveMessage):
		return messageReceiveEventHandler(v)
	case func(*Session, *FriendRequest):
		return friendRequestEventHandler(v)
	case func(*Session, *BotOffline):
		return botOfflineEventHandler(v)
	case func(*Session, *FriendNudge):
		return friendNudgeEventHandler(v)
	case func(*Session, *FriendFileUpload):
		return friendFileUploadEventHandler(v)
	case func(*Session, *GroupNudge):
		return groupNudgeEventHandler(v)
	case func(*Session, *GroupMessageReaction):
		return groupMessageReactionEventHandler(v)
	case func(*Session, *GroupMute):
		return groupMuteEventHandler(v)
	case func(*Session, *GroupWholeMute):
		return groupWholeMuteEventHandler(v)
	case func(*Session, *GroupMemberIncrease):
		return groupMemberIncreaseEventHandler(v)
	case func(*Session, *GroupMemberDecrease):
		return groupMemberDecreaseEventHandler(v)
	case func(*Session, *GroupInvitation):
		return groupInvitationEventHandler(v)
	case func(*Session, *GroupJoinRequest):
		return groupJoinRequestEventHandler(v)
	case func(*Session, *GroupInvitedJoinRequest):
		return groupInvitedJoinRequestEventHandler(v)
	case func(*Session, *MessageRecall):
		return messageRecallEventHandler(v)
	case func(*Session, *GroupAdminChange):
		return groupAdminChangeEventHandler(v)
	case func(*Session, *GroupEssenceMessageChange):
		return groupEssenceMessageChangeEventHandler(v)
	case func(*Session, *GroupNameChange):
		return groupNameChangeEventHandler(v)
	case func(*Session, *GroupFileUpload):
		return groupFileUploadEventHandler(v)
	default:
		return nil
	}
}

// messageReceiveEventHandler is an event handler for ReceiveMessage events.
type messageReceiveEventHandler func(*Session, *ReceiveMessage)

// Type returns the event type for MessageCreate events.
func (eh messageReceiveEventHandler) Type() string {
	return messageReceiveEventType
}

// New returns a new instance of MessageCreate.
func (eh messageReceiveEventHandler) New() interface{} {
	return &ReceiveMessage{}
}

// Handle is the handler for MessageCreate events.
func (eh messageReceiveEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*ReceiveMessage); ok {
		eh(s, t)
	}
}

type friendRequestEventHandler func(*Session, *FriendRequest)

// Type returns the event type for FriendRequest events.
func (eh friendRequestEventHandler) Type() string {
	return friendRequestEventType
}

// New returns a new instance of FriendRequest.
func (eh friendRequestEventHandler) New() interface{} {
	return &FriendRequest{}
}

// Handle is the handler for FriendRequest events.
func (eh friendRequestEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*FriendRequest); ok {
		eh(s, t)
	}
}

type botOfflineEventHandler func(*Session, *BotOffline)

func (eh botOfflineEventHandler) Handle(session *Session, i interface{}) {
	if t, ok := i.(*BotOffline); ok {
		eh(session, t)
	}
}

func (eh botOfflineEventHandler) Type() string {
	return botOfflineEventType
}

func (eh botOfflineEventHandler) New() interface{} {
	return &BotOffline{}
}

type groupNudgeEventHandler func(*Session, *GroupNudge)

func (eh groupNudgeEventHandler) Type() string {
	return groupNudgeEventType
}

func (eh groupNudgeEventHandler) New() interface{} {
	return &GroupNudge{}
}

func (eh groupNudgeEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupNudge); ok {
		eh(s, t)
	}
}

type groupMessageReactionEventHandler func(*Session, *GroupMessageReaction)

func (eh groupMessageReactionEventHandler) Type() string {
	return groupMessageReactionEventType
}

func (eh groupMessageReactionEventHandler) New() interface{} {
	return &GroupMessageReaction{}
}

func (eh groupMessageReactionEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupMessageReaction); ok {
		eh(s, t)
	}
}

type groupMuteEventHandler func(*Session, *GroupMute)

func (eh groupMuteEventHandler) Type() string {
	return groupMuteEventType
}

func (eh groupMuteEventHandler) New() interface{} {
	return &GroupMute{}
}

func (eh groupMuteEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupMute); ok {
		eh(s, t)
	}
}

type groupWholeMuteEventHandler func(*Session, *GroupWholeMute)

func (eh groupWholeMuteEventHandler) Type() string {
	return groupWholeMuteEventType
}

func (eh groupWholeMuteEventHandler) New() interface{} {
	return &GroupWholeMute{}
}

func (eh groupWholeMuteEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupWholeMute); ok {
		eh(s, t)
	}
}

type groupMemberIncreaseEventHandler func(*Session, *GroupMemberIncrease)

func (eh groupMemberIncreaseEventHandler) Type() string {
	return groupMemberIncreaseEventType
}

func (eh groupMemberIncreaseEventHandler) New() interface{} {
	return &GroupMemberIncrease{}
}

func (eh groupMemberIncreaseEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupMemberIncrease); ok {
		eh(s, t)
	}
}

// groupMemberDecreaseEventHandler is an event handler for GroupMemberDecrease events.
type groupMemberDecreaseEventHandler func(*Session, *GroupMemberDecrease)

func (eh groupMemberDecreaseEventHandler) Type() string {
	return groupMemberDecreaseEventType
}

func (eh groupMemberDecreaseEventHandler) New() interface{} {
	return &GroupMemberDecrease{}
}

func (eh groupMemberDecreaseEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupMemberDecrease); ok {
		eh(s, t)
	}
}

// groupInvitationEventHandler is an event handler for GroupInvitation events.
type groupInvitationEventHandler func(*Session, *GroupInvitation)

func (eh groupInvitationEventHandler) Type() string {
	return groupInvitationEventType
}

func (eh groupInvitationEventHandler) New() interface{} {
	return &GroupInvitation{}
}

func (eh groupInvitationEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupInvitation); ok {
		eh(s, t)
	}
}

type groupJoinRequestEventHandler func(*Session, *GroupJoinRequest)

func (eh groupJoinRequestEventHandler) Type() string {
	return groupJoinRequestEventType
}

func (eh groupJoinRequestEventHandler) New() interface{} {
	return &GroupJoinRequest{}
}

func (eh groupJoinRequestEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupJoinRequest); ok {
		eh(s, t)
	}
}

type groupInvitedJoinRequestEventHandler func(*Session, *GroupInvitedJoinRequest)

func (eh groupInvitedJoinRequestEventHandler) Type() string {
	return groupInvitedJoinRequestEventType
}

func (eh groupInvitedJoinRequestEventHandler) New() interface{} {
	return &GroupInvitedJoinRequest{}
}

func (eh groupInvitedJoinRequestEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupInvitedJoinRequest); ok {
		eh(s, t)
	}
}

type groupAdminChangeEventHandler func(*Session, *GroupAdminChange)

func (eh groupAdminChangeEventHandler) Type() string {
	return groupAdminChangeEventType
}

func (eh groupAdminChangeEventHandler) New() interface{} {
	return &GroupAdminChange{}
}

func (eh groupAdminChangeEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupAdminChange); ok {
		eh(s, t)
	}
}

type groupEssenceMessageChangeEventHandler func(*Session, *GroupEssenceMessageChange)

func (eh groupEssenceMessageChangeEventHandler) Type() string {
	return groupEssenceMessageChangeEventType
}

func (eh groupEssenceMessageChangeEventHandler) New() interface{} {
	return &GroupEssenceMessageChange{}
}

func (eh groupEssenceMessageChangeEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupEssenceMessageChange); ok {
		eh(s, t)
	}
}

type groupNameChangeEventHandler func(*Session, *GroupNameChange)

func (eh groupNameChangeEventHandler) Type() string {
	return groupNameChangeEventType
}

func (eh groupNameChangeEventHandler) New() interface{} {
	return &GroupNameChange{}
}

func (eh groupNameChangeEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupNameChange); ok {
		eh(s, t)
	}
}

type groupFileUploadEventHandler func(*Session, *GroupFileUpload)

func (eh groupFileUploadEventHandler) Type() string {
	return groupFileUploadEventType
}

func (eh groupFileUploadEventHandler) New() interface{} {
	return &GroupFileUpload{}
}

func (eh groupFileUploadEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*GroupFileUpload); ok {
		eh(s, t)
	}
}

type messageRecallEventHandler func(*Session, *MessageRecall)

func (eh messageRecallEventHandler) Type() string {
	return messageRecallEventType
}

func (eh messageRecallEventHandler) New() interface{} {
	return &MessageRecall{}
}

func (eh messageRecallEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*MessageRecall); ok {
		eh(s, t)
	}
}

type friendNudgeEventHandler func(*Session, *FriendNudge)

func (eh friendNudgeEventHandler) Type() string {
	return friendNudgeEventType
}

func (eh friendNudgeEventHandler) New() interface{} {
	return &FriendNudge{}
}

func (eh friendNudgeEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*FriendNudge); ok {
		eh(s, t)
	}
}

type friendFileUploadEventHandler func(*Session, *FriendFileUpload)

func (eh friendFileUploadEventHandler) Type() string {
	return friendFileUploadEventType
}

func (eh friendFileUploadEventHandler) New() interface{} {
	return &FriendFileUpload{}
}

func (eh friendFileUploadEventHandler) Handle(s *Session, i interface{}) {
	if t, ok := i.(*FriendFileUpload); ok {
		eh(s, t)
	}
}

func init() {
	registerInterfaceProvider(messageReceiveEventHandler(nil))
	registerInterfaceProvider(friendRequestEventHandler(nil))
	registerInterfaceProvider(botOfflineEventHandler(nil))
	registerInterfaceProvider(friendNudgeEventHandler(nil))
	registerInterfaceProvider(groupNudgeEventHandler(nil))
	registerInterfaceProvider(groupMuteEventHandler(nil))
	registerInterfaceProvider(groupWholeMuteEventHandler(nil))
	registerInterfaceProvider(groupJoinRequestEventHandler(nil))
	registerInterfaceProvider(groupInvitedJoinRequestEventHandler(nil))
	registerInterfaceProvider(groupAdminChangeEventHandler(nil))
	registerInterfaceProvider(groupEssenceMessageChangeEventHandler(nil))
	registerInterfaceProvider(groupNameChangeEventHandler(nil))
	registerInterfaceProvider(groupFileUploadEventHandler(nil))
	registerInterfaceProvider(groupMessageReactionEventHandler(nil))
	registerInterfaceProvider(groupMemberIncreaseEventHandler(nil))
	registerInterfaceProvider(groupMemberDecreaseEventHandler(nil))
	registerInterfaceProvider(groupInvitationEventHandler(nil))
	registerInterfaceProvider(messageRecallEventHandler(nil))
	registerInterfaceProvider(friendFileUploadEventHandler(nil))
}
