package messages

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func MarkAsRead(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	startID, _ := strconv.ParseUint(c.Query("start_message_id"), 10, 64)
	idsStr := c.Query("message_ids")

	if peerID == 0 && idsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_ids")
		return
	}

	type markTask struct {
		maxLocalID uint64
		pID        int64
	}
	tasks := make(map[string]*markTask)

	if peerID != 0 && startID != 0 {
		chatID := chat.GetInternalChatID(peerID, currentUserID)
		tasks[chatID] = &markTask{maxLocalID: startID, pID: peerID}
	}

	if idsStr != "" {
		parts := strings.Split(idsStr, ",")
		for _, p := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
				var msg db_models.Message
				if err := db.Instance.Select("chat_id", "local_id").Where("id = ?", id).First(&msg).Error; err == nil {
					if tasks[msg.ChatID] == nil {
						tasks[msg.ChatID] = &markTask{
							maxLocalID: msg.LocalID,
							pID:        chat.DerivePeerID(msg.ChatID, currentUserID),
						}
					} else if msg.LocalID > tasks[msg.ChatID].maxLocalID {
						tasks[msg.ChatID].maxLocalID = msg.LocalID
					}
				}
			}
		}
	}

	if peerID != 0 && startID == 0 && idsStr == "" {
		chatID := chat.GetInternalChatID(peerID, currentUserID)
		conv, _ := chat.GetConversation(db.Instance, chatID)
		if conv != nil {
			tasks[chatID] = &markTask{maxLocalID: conv.LastMessageID, pID: peerID}
		}
	}

	if len(tasks) > 0 {
		go func(tks map[string]*markTask, uid int64) {
			ctx := context.Background()

			for cID, task := range tks {
				if err := chat.MarkAsRead(db.Instance, cID, uid, task.maxLocalID); err == nil {
					r.BroadcastMarkAsRead(ctx, task.pID, uid, task.maxLocalID)
				}
			}
		}(tasks, currentUserID)
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
