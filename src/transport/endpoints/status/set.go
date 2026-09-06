package status

import (
	"context"
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat" // НОВОЕ: Добавлен импорт репозитория чата
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings" // НОВОЕ: Добавлен импорт для проверки префиксов

	"github.com/gin-gonic/gin"
)

func SetActivity(c *gin.Context, r *core.BaseHandler) {
	userIDVal, _ := c.Get("userID")
	currentUserID := userIDVal.(int64)

	TouchUserActivity(c.Request.Context(), r.DB, r.LPRepo.Client, r.LPRepo, r.Broadcaster, currentUserID)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		if uID := c.Query("user_id"); uID != "" {
			peerID, _ = strconv.ParseInt(uID, 10, 64)
		}
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: user_id or peer_id")
		return
	}

	activityType := c.DefaultQuery("type", "typing")
	var flag uint8 = 1
	if activityType == "audiomessage" {
		flag = 2
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	isGroupChat := strings.HasPrefix(internalChatID, "c")

	var eventType string
	var event lp_models.VKEvent

	if isGroupChat {
		eventType = "is_chat_typing"
		event = &lp_models.IsChatTypingEvent{
			UserID: currentUserID,
			ChatID: peerID - 2000000000,
			Flags:  flag,
		}
	} else {
		eventType = "is_dm_typing"
		event = &lp_models.IsDMTypingEvent{
			UserID: currentUserID,
			Flags:  flag,
		}
	}

	go func(pID, senderID int64, chID string, groupChat bool, t string, ev lp_models.VKEvent) {
		ctx := context.Background()
		var recipients []int64

		if groupChat {
			dbx.Instance.Model(&db_models.ConversationMember{}).
				Where("internal_chat_id = ? AND user_id != ? AND left_at IS NULL", chID, senderID).
				Pluck("user_id", &recipients)
		} else if pID != senderID {
			recipients = []int64{pID}
		}

		for _, uid := range recipients {
			_ = r.LPRepo.PushEphemeralEvent(ctx, uid, t, ev)
			r.Broadcaster.Notify(uid)
		}
	}(peerID, currentUserID, internalChatID, isGroupChat, eventType, event)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func pushActivity(c *gin.Context, r *core.BaseHandler, recipients []int64, eventType string, event lp_models.VKEvent) {
	for _, uid := range recipients {
		_, _, err := r.LPRepo.PushEvent(c.Request.Context(), uid, eventType, event)
		if err == nil {
			r.Broadcaster.Notify(uid)
		}
	}
}
