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

func DeleteConversation(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		if uID, err := strconv.ParseInt(c.Query("user_id"), 10, 64); err == nil && uID != 0 {
			peerID = uID
		} else if cID, err := strconv.ParseInt(c.Query("chat_id"), 10, 64); err == nil && cID != 0 {
			if cID > 2000000000 {
				peerID = cID
			} else {
				peerID = 2000000000 + cID
			}
		}
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)

	var member db_models.ConversationMember
	err := db.Instance.Where("user_id = ? AND internal_chat_id = ?", currentUserID, internalChatID).First(&member).Error
	if err != nil {
		r.Reject(c, 917, "You don't have access to this chat")
		return
	}

	// Synchronously execute database deletion so immediate follow-up reads see updated deleted_before_id
	r.BackgroundDeleteChat(currentUserID, peerID, member.InternalChatID)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
