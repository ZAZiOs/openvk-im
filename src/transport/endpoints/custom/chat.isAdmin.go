package custom

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func IsChatAdminHandler(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")

	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	internalChatId := chat.GetInternalChatID(peerID, currentUserID)

	var count int64
	err := db.Instance.Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND user_id = ? AND is_admin = ? AND left_at IS NULL",
			internalChatId, currentUserID, true).
		Count(&count).Error

	if err != nil {
		r.Reject(c, 500, "Internal server error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"is_admin": count > 0,
		},
	})
}
