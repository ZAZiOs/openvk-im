package chats

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetConversations(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}

	filter := c.DefaultQuery("filter", "all")
	extended := c.Query("extended") == "1"

	// бля это пиздец.
	query := db.Instance.Table("conversation_members").
		Select("conversation_members.*, conversations.last_message_id as conv_last_id").
		Joins("JOIN conversations ON conversations.internal_id = conversation_members.internal_chat_id").
		Where("conversation_members.user_id = ? AND conversation_members.left_at IS NULL", currentUserID)

	if filter == "unread" {
		query = query.Where("conversations.last_message_id > conversation_members.last_read_id")
	}

	query = query.Order("conversations.last_message_id DESC")

	var totalCount int64
	query.Count(&totalCount)

	var memberships []db_models.ConversationMember
	err := query.Limit(count).Offset(offset).Find(&memberships).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	var totalUnreadConversations int64
	db.Instance.Table("conversation_members").
		Joins("JOIN conversations ON conversations.internal_id = conversation_members.internal_chat_id").
		Where("conversation_members.user_id = ? AND conversation_members.left_at IS NULL", currentUserID).
		Where("conversations.last_message_id > conversation_members.last_read_id").
		Count(&totalUnreadConversations)

	responseItems := make([]gin.H, 0)
	var userIDs []int64
	var groupIDs []int64

	for _, m := range memberships {
		var conv db_models.Conversation
		db.Instance.Where("internal_id = ?", m.InternalChatID).First(&conv)

		pID := chat.DerivePeerID(m.InternalChatID, currentUserID)

		var lastMsg db_models.Message
		var msgVK interface{} = nil
		if conv.LastMessageID > 0 {
			db.Instance.Where("chat_id = ? AND local_id = ?", m.InternalChatID, conv.LastMessageID).First(&lastMsg)
			msgVK = lastMsg.ToVKApiStruct(db.Instance, 0, currentUserID, pID)
		}

		conversationObj := gin.H{
			"peer": gin.H{
				"id":   pID,
				"type": getPeerType(pID),
			},
			"last_message_id": conv.LastMessageID,
			"in_read":         m.LastReadID,
			"out_read":        conv.LastMessageID,
		}

		if conv.LastMessageID > m.LastReadID {
			var unreadCount int64
			db.Instance.Model(&db_models.Message{}).
				Where("chat_id = ? AND local_id > ? AND from_id != ?", m.InternalChatID, m.LastReadID, currentUserID).
				Count(&unreadCount)
			conversationObj["unread_count"] = unreadCount
		}

		if conv.PinnedMsgID > 0 {
			conversationObj["current_pinned_message"] = gin.H{"id": conv.PinnedMsgID}
		}

		responseItems = append(responseItems, gin.H{
			"conversation": conversationObj,
			"last_message": msgVK,
		})

		if extended {
			if lastMsg.FromID > 0 {
				userIDs = append(userIDs, lastMsg.FromID)
			} else if lastMsg.FromID < 0 {
				groupIDs = append(groupIDs, -lastMsg.FromID)
			}
			if pID > 0 && pID < 2000000000 {
				userIDs = append(userIDs, pID)
			} else if pID < 0 {
				groupIDs = append(groupIDs, -pID)
			}
		}
	}

	result := gin.H{
		"count":        totalCount,
		"items":        responseItems,
		"unread_count": totalUnreadConversations,
	}

	if extended {
		result["profiles"] = uniqueIDs(userIDs)
		result["groups"] = uniqueIDs(groupIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}

func getPeerType(peerID int64) string {
	if peerID > 2000000000 {
		return "chat"
	}
	if peerID < 0 {
		return "community"
	}
	return "user"
}

func uniqueIDs(ids []int64) []int64 {
	m := make(map[int64]bool)
	var res []int64
	for _, id := range ids {
		if !m[id] {
			m[id] = true
			res = append(res, id)
		}
	}
	return res
}
