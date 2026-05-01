package status

import (
	"context"
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func SetActivity(c *gin.Context, r *core.BaseHandler) {
	userIDVal, _ := c.Get("userID")
	currentUserID := userIDVal.(int64)

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

	var eventType string
	var recipients []int64
	var event lp_models.VKEvent

	if peerID > 2000000000 {
		eventType = "is_chat_typing"
		event = &lp_models.IsChatTypingEvent{
			UserID: currentUserID,
			ChatID: peerID - 2000000000,
			Flags:  flag,
		}

		dbx.Instance.Model(&db_models.ConversationMember{}).
			Where("peer_id = ? AND user_id != ?", peerID, currentUserID).
			Pluck("user_id", &recipients)
	} else {
		eventType = "is_dm_typing"
		event = &lp_models.IsDMTypingEvent{
			UserID: currentUserID,
			Flags:  flag,
		}

		if peerID != currentUserID {
			recipients = []int64{peerID}
		}
	}

	if len(recipients) > 0 {
		go func(uids []int64, t string, ev lp_models.VKEvent) {
			for _, uid := range uids {
				_ = r.LPRepo.PushEphemeralEvent(context.Background(), uid, t, ev)
				r.Broadcaster.Notify(uid)
			}
		}(recipients, eventType, event)
	}

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
