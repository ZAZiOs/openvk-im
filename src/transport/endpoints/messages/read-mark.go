package messages

import (
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

	if peerID != 0 && startID != 0 {
		if err := chat.MarkAsRead(db.Instance, peerID, currentUserID, startID); err == nil {
			r.BroadcastMarkAsRead(c, peerID, currentUserID, startID)
		}
	}

	if idsStr != "" {
		parts := strings.Split(idsStr, ",")
		var maxID uint64
		var foundPeerID int64

		for _, p := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
				if peerID == 0 {
					var msg db_models.Message
					db.Instance.Select("peer_id", "local_id").Where("id = ?", id).First(&msg)
					foundPeerID = msg.PeerID
					if msg.LocalID > maxID {
						maxID = msg.LocalID
					}
				} else {
					foundPeerID = peerID
					var msg db_models.Message
					db.Instance.Select("local_id").Where("id = ? AND peer_id = ?", id, peerID).First(&msg)
					if msg.LocalID > maxID {
						maxID = msg.LocalID
					}
				}
			}
		}

		if foundPeerID != 0 && maxID != 0 {
			if err := chat.MarkAsRead(db.Instance, foundPeerID, currentUserID, maxID); err == nil {
				r.BroadcastMarkAsRead(c, foundPeerID, currentUserID, maxID)
			}
		}
	}

	if peerID != 0 && startID == 0 && idsStr == "" {
		conv, _ := chat.GetConversation(db.Instance, peerID)
		if conv != nil {
			chat.MarkAsRead(db.Instance, peerID, currentUserID, conv.LastMessageID)
		}
	}

	if peerID != 0 && startID == 0 && idsStr == "" {
		conv, _ := chat.GetConversation(db.Instance, peerID)
		if conv != nil {
			if err := chat.MarkAsRead(db.Instance, peerID, currentUserID, conv.LastMessageID); err == nil {
				r.BroadcastMarkAsRead(c, peerID, currentUserID, conv.LastMessageID)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
