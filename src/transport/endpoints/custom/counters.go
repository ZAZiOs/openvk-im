package custom

import (
	"net/http"
	"ovk-im/src/db"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetCounters(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)

	var totalUnread int64

	query := `
        SELECT COUNT(m.id) 
        FROM messages m
        JOIN conversation_members cm ON cm.internal_chat_id = m.chat_id
        JOIN conversations conv ON conv.internal_id = m.chat_id
        WHERE cm.user_id = ? 
          AND cm.left_at IS NULL 
          AND m.local_id > cm.last_read_id
          AND m.from_id != ?
    `

	err := db.Instance.Raw(query, userID, userID).Scan(&totalUnread).Error

	if err != nil {
		r.Reject(c, 500, "Database error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"messages": totalUnread,
		},
	})
}
