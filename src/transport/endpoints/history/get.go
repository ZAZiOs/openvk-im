package history

import (
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetHistory(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if uID := c.Query("user_id"); uID != "" {
		id, _ := strconv.ParseInt(uID, 10, 64)
		peerID = id
	} else if chatID := c.Query("chat_id"); chatID != "" {
		id, _ := strconv.ParseInt(chatID, 10, 64)
		peerID = 2000000000 + id
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id, user_id or chat_id")
		return
	}

	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	rev, _ := strconv.Atoi(c.DefaultQuery("rev", "0")) // 0 - desc, 1 - asc

	if peerID > 2000000000 {
		inChat, err := chat.IsUserInChat(nil, peerID, currentUserID)
		if err != nil || !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	var msgs []db_models.Message
	query := db.Instance.Where("chat_id = ?", peerID)

	order := "local_id DESC"
	if rev == 1 {
		order = "local_id ASC"
	}

	err := query.Order(order).Limit(count).Offset(offset).Find(&msgs).Error
	if err != nil {
		r.Reject(c, 10, "Internal server error during DB query")
		return
	}

	var totalCount int64
	db.Instance.Model(&db_models.Message{}).Where("chat_id = ?", peerID).Count(&totalCount)

	responseItems := make([]db_models.VKApiMessage, len(msgs))
	for i, m := range msgs {
		responseItems[i] = m.ToVKApiStruct(db.Instance, 1, currentUserID)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"count":  totalCount,
			"items":  responseItems,
			"unread": 0,
		},
	})
}
