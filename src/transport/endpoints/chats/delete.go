package chats

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func DeleteConversation(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	var member db_models.ConversationMember
	err := db.Instance.Where("user_id = ? AND peer_id = ?", currentUserID, peerID).First(&member).Error
	if err != nil {
		r.Reject(c, 917, "You don't have access to this chat")
		return
	}

	go r.BackgroundDeleteChat(currentUserID, peerID, member.InternalChatID)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
