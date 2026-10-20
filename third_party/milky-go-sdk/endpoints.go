package Milky_go_sdk

import "fmt"

type apiEndpoints struct {
	Gateway string
}

// NewAPIEndpoints 创建一个新的 apiEndpoints 实例
func newAPIEndpoints(gateway string) *apiEndpoints {
	return &apiEndpoints{Gateway: gateway}
}

// Endpoint 生成完整的 API 端点
func (e *apiEndpoints) Endpoint(path string) string {
	return fmt.Sprintf("%s/%s", e.Gateway, path)
}

const (
	// EndpointGetLoginInfo System

	EndpointGetLoginInfo       = "get_login_info"
	EndpointGetImplInfo        = "get_impl_info"
	EndpointGetFriendList      = "get_friend_list"
	EndpointGetUserProfile     = "get_user_profile"
	EndpointGetFriendInfo      = "get_friend_info"
	EndpointGetGroupList       = "get_group_list"
	EndpointGetGroupInfo       = "get_group_info"
	EndpointGetGroupMemberList = "get_group_member_list"
	EndpointGetGroupMemberInfo = "get_group_member_info"
	EndpointGetCookies         = "get_cookies"
	EndpointGetCSRFToken       = "get_csrf_token"

	// EndpointSendPrivateMessage Message

	EndpointSendPrivateMessage   = "send_private_message"
	EndpointSendGroupMessage     = "send_group_message"
	EndpointGetMessage           = "get_message"
	EndpointGetHistoryMessages   = "get_history_messages"
	EndpointGetResourceTempURL   = "get_resource_temp_url"
	EndpointGetForwardedMessages = "get_forwarded_messages"
	EndpointRecallPrivateMessage = "recall_private_message"
	EndpointRecallGroupMessage   = "recall_group_message"
	EndpointMarkMessageAsRead    = "mark_message_as_read"

	// EndpointSendFriendNudge Friend

	EndpointSendFriendNudge     = "send_friend_nudge"
	EndpointSendProfileLike     = "send_profile_like"
	EndpointGetFriendRequests   = "get_friend_requests"
	EndpointAcceptFriendRequest = "accept_friend_request"
	EndpointRejectFriendRequest = "reject_friend_request"

	// EndpointSetGroupName Group

	EndpointSetGroupName               = "set_group_name"
	EndpointSetGroupAvatar             = "set_group_avatar"
	EndpointSetGroupMemberCard         = "set_group_member_card"
	EndpointSetGroupMemberSpecialTitle = "set_group_member_special_title"
	EndpointSetGroupMemberAdmin        = "set_group_member_admin"
	EndpointSetGroupMemberMute         = "set_group_member_mute"
	EndpointSetGroupWholeMute          = "set_group_whole_mute"
	EndpointKickGroupMember            = "kick_group_member"
	EndpointGetGroupAnnouncementList   = "get_group_announcement_list"
	EndpointSendGroupAnnouncement      = "send_group_announcement"
	EndpointDeleteGroupAnnouncement    = "delete_group_announcement"
	EndpointGetGroupEssenceMessages    = "get_group_essence_messages"
	EndpointSetGroupEssenceMessage     = "set_group_essence_message"
	EndpointQuitGroup                  = "quit_group"
	EndpointSendGroupMessageReaction   = "send_group_message_reaction"
	EndpointSendGroupNudge             = "send_group_nudge"
	EndpointRejectGroupInvitation      = "reject_group_invitation"
	EndpointAcceptGroupInvitation      = "accept_group_invitation"
	EndpointAcceptGroupRequest         = "accept_group_request"
	EndpointRejectGroupRequest         = "reject_group_request"
	EndpointGetGroupNotifications      = "get_group_notifications"

	// EndpointUploadPrivateFile File

	EndpointUploadPrivateFile         = "upload_private_file"
	EndpointUploadGroupFile           = "upload_group_file"
	EndpointGetPrivateFileDownloadURL = "get_private_file_download_url"
	EndpointGetGroupFileDownloadURL   = "get_group_file_download_url"
	EndpointGetGroupFiles             = "get_group_files"
	EndpointMoveGroupFile             = "move_group_file"
	EndpointRenameGroupFile           = "rename_group_file"
	EndpointDeleteGroupFile           = "delete_group_file"
	EndpointCreateGroupFolder         = "create_group_folder"
	EndpointRenameGroupFolder         = "rename_group_folder"
	EndpointDeleteGroupFolder         = "delete_group_folder"
)
