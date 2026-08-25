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

func Pin(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	messageID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64) // Это local_id от клиента

	if peerID == 0 || messageID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, chatID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	} else {
		inChat, _ := chat.IsUserInChat(db.Instance, chatID, currentUserID)
		if !inChat {
			r.Reject(c, 917, "Access denied")
			return
		}
	}

	var msg db_models.Message
	q := db.Instance.Where("chat_id = ? AND local_id = ?", chatID, messageID)
	q = db_models.BuildVisibilityFilter(q, chatID, currentUserID)
	if err := q.First(&msg).Error; err != nil {
		r.Reject(c, 946, "Message not found")
		return
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("internal_id = ?", chatID).
		Update("pinned_msg_id", messageID).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"response": msg.ToVKApiStruct(db.Instance, 0, currentUserID, peerID),
	})
}

func Unpin(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, chatID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("internal_id = ?", chatID).
		Update("pinned_msg_id", 0).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
