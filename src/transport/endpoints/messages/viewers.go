package messages

import (
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

type MessageViewerItem struct {
	UserID     int64  `json:"user_id"`
	LastReadID uint64 `json:"last_read_id"`
}

func GetMessageViewers(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	messageID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)
	cmid, _ := strconv.ParseUint(c.Query("conversation_message_id"), 10, 64)
	if cmid == 0 {
		cmid, _ = strconv.ParseUint(c.Query("cmid"), 10, 64)
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	if messageID == 0 && cmid == 0 {
		r.Reject(c, 100, "One of the parameters is missing: message_id or conversation_message_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	inChat, _ := chat.IsUserInChat(db.Instance, chatID, currentUserID)
	if !inChat {
		r.Reject(c, 917, "Access denied")
		return
	}

	var msg db_models.Message
	q := db.Instance.Where("chat_id = ?", chatID)
	if cmid > 0 {
		q = q.Where("local_id = ?", cmid)
	} else {
		q = q.Where("local_id = ? OR id = ?", messageID, messageID)
	}
	q = db_models.BuildVisibilityFilter(q, chatID, currentUserID)

	if err := q.First(&msg).Error; err != nil {
		r.Reject(c, 100, "Message not found")
		return
	}

	if msg.FromID != currentUserID {
		r.Reject(c, 917, "Access denied: you can only view viewers of your own messages")
		return
	}

	type MemberRow struct {
		UserID     int64  `gorm:"column:user_id"`
		LastReadID uint64 `gorm:"column:last_read_id"`
	}

	var memberRows []MemberRow
	membersQuery := db.Instance.Table("conversation_members").
		Select("conversation_members.user_id, conversation_members.last_read_id").
		Where("conversation_members.internal_chat_id = ?", chatID).
		Where("conversation_members.last_read_id >= ?", msg.LocalID).
		Where("conversation_members.user_id != ?", msg.FromID).
		Where(`(
			conversation_members.internal_chat_id NOT LIKE 'c%' 
			OR NOT EXISTS (SELECT 1 FROM conversation_member_periods p0 WHERE p0.internal_chat_id = conversation_members.internal_chat_id AND p0.user_id = conversation_members.user_id) 
			OR EXISTS (SELECT 1 FROM conversation_member_periods p WHERE p.internal_chat_id = conversation_members.internal_chat_id AND p.user_id = conversation_members.user_id AND ? >= p.start_local_id AND (p.end_local_id IS NULL OR ? <= p.end_local_id))
		)`, msg.LocalID, msg.LocalID)

	if err := membersQuery.Order("conversation_members.last_read_id DESC").Find(&memberRows).Error; err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	userIDs := make([]int64, 0, len(memberRows))
	items := make([]MessageViewerItem, 0, len(memberRows))
	for _, row := range memberRows {
		userIDs = append(userIDs, row.UserID)
		items = append(items, MessageViewerItem{
			UserID:     row.UserID,
			LastReadID: row.LastReadID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"count":    len(items),
			"user_ids": userIDs,
			"items":    items,
		},
	})
}
