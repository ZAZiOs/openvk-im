package custom

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
)

func CheckPeerExist(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		r.Reject(c, 5, "User authorization failed: invalid session")
		return
	}
	currentUserID := val.(int64)

	var peerID int64
	if pID := c.DefaultQuery("peer_id", c.PostForm("peer_id")); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if uID := c.DefaultQuery("user_id", c.PostForm("user_id")); uID != "" {
		id, _ := strconv.ParseInt(uID, 10, 64)
		peerID = id
	} else if chatID := c.DefaultQuery("chat_id", c.PostForm("chat_id")); chatID != "" {
		id, _ := strconv.ParseInt(chatID, 10, 64)
		peerID = 2000000000 + id
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	isGroupChat := strings.HasPrefix(internalChatID, "c")

	var member db_models.ConversationMember
	err := db.Instance.Where("internal_chat_id = ? AND user_id = ? AND left_at IS NULL", internalChatID, currentUserID).First(&member).Error

	peerExists := false
	if err == nil {
		if isGroupChat {
			peerExists = true
		} else {
			if member.LastMessageID > member.DeletedBeforeID {
				var count int64
				q := db.Instance.Model(&db_models.Message{}).Where("chat_id = ?", internalChatID)
				q = db_models.BuildVisibilityFilter(q, internalChatID, currentUserID)
				if countErr := q.Count(&count).Error; countErr == nil && count > 0 {
					peerExists = true
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"exists": peerExists,
		},
	})
}
