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
	messageID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)

	if peerID == 0 || messageID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_id")
		return
	}

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, peerID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	}

	var msg db_models.Message
	if err := db.Instance.Where("peer_id = ? AND local_id = ?", peerID, messageID).First(&msg).Error; err != nil {
		r.Reject(c, 946, "Message not found")
		return
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("peer_id = ?", peerID).
		Update("pinned_msg_id", messageID).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"response": msg.ToVKApiStruct(db.Instance, 0, currentUserID),
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

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, peerID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("peer_id = ?", peerID).
		Update("pinned_msg_id", 0).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"response": 1,
	})
}
