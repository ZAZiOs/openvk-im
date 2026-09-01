package messages

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetNearestMessageForDate(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	var uIDParam int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	}
	if uID := c.Query("user_id"); uID != "" {
		uIDParam, _ = strconv.ParseInt(uID, 10, 64)
		if peerID == 0 {
			peerID = uIDParam
		}
	} else if chatIDParam := c.Query("chat_id"); chatIDParam != "" {
		if id, err := strconv.ParseInt(chatIDParam, 10, 64); err == nil && peerID == 0 {
			peerID = 2000000000 + id
		}
	}

	chatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)
	if chatID == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id, user_id or chat_id")
		return
	}

	if peerID == 0 {
		peerID = chat.DerivePeerID(chatID, currentUserID)
	}

	isGroupChat := strings.HasPrefix(chatID, "c")

	if currentUserID != 0 {
		if isGroupChat {
			member, err := chat.GetMember(db.Instance, chatID, currentUserID)
			if err != nil || member == nil || member.LeftAt != nil {
				r.Reject(c, 917, "You don't have access to this chat")
				return
			}
		} else {
			member, _ := chat.GetMember(db.Instance, chatID, currentUserID)
			if member == nil {
				r.Reject(c, 917, "Conversation doesn't exist")
				return
			}
		}
	}

	dateParam := c.Query("date")
	var targetTime time.Time
	if ts, err := strconv.ParseInt(dateParam, 10, 64); err == nil && ts > 0 {
		targetTime = time.Unix(ts, 0)
	} else if dateParam != "" {
		if t, err := time.Parse("2006-01-02", dateParam); err == nil {
			targetTime = t
		} else if t, err := time.Parse(time.RFC3339, dateParam); err == nil {
			targetTime = t
		} else if t, err := time.Parse("02.01.2006", dateParam); err == nil {
			targetTime = t
		} else {
			targetTime = time.Now()
		}
	} else {
		targetTime = time.Now()
	}

	var msg db_models.Message
	query := db.Instance.Where("chat_id = ?", chatID)
	query = db_models.BuildVisibilityFilter(query, chatID, currentUserID)

	// Try to find the first message on or after target date
	err := query.Where("created_at >= ?", targetTime).Order("created_at ASC, local_id ASC").Limit(1).Find(&msg).Error
	if err != nil || msg.ID == 0 {
		// If no message after target date, find the closest message before target date
		var fallbackMsg db_models.Message
		fbQuery := db.Instance.Where("chat_id = ?", chatID)
		fbQuery = db_models.BuildVisibilityFilter(fbQuery, chatID, currentUserID)
		if fbErr := fbQuery.Where("created_at < ?", targetTime).Order("created_at DESC, local_id DESC").Limit(1).Find(&fallbackMsg).Error; fbErr == nil && fallbackMsg.ID != 0 {
			msg = fallbackMsg
			err = nil
		}
	}

	if msg.ID == 0 {
		c.JSON(http.StatusOK, gin.H{
			"response": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"id":                      msg.ID,
			"peer_id":                 peerID,
			"date":                    msg.CreatedAt.Unix(),
			"conversation_message_id": msg.LocalID,
		},
	})
}
