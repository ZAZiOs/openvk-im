package lp_ep

import (
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	redis_repo "ovk-im/src/repo/redis"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetLongPollHistory(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)
	apiV := core.GetApiV(c)

	ts, _ := strconv.ParseUint(c.Query("ts"), 10, 64)
	pts, _ := strconv.ParseUint(c.Query("pts"), 10, 64)
	eventsLimit, _ := strconv.Atoi(c.DefaultQuery("events_limit", "1000"))
	msgsLimit, _ := strconv.Atoi(c.DefaultQuery("msgs_limit", "200"))
	if msgsLimit > 1000 {
		msgsLimit = 1000
	} else if msgsLimit < 1 {
		msgsLimit = 200
	}

	previewLen, _ := strconv.Atoi(c.DefaultQuery("preview_length", "0"))
	maxMsgID, _ := strconv.ParseUint(c.DefaultQuery("max_msg_id", "0"), 10, 64)

	version, _ := strconv.Atoi(c.DefaultQuery("version", "2"))
	if v := c.Query("lp_version"); v != "" {
		if ver, err := strconv.Atoi(v); err == nil {
			version = ver
		}
	}
	mode, _ := strconv.Atoi(c.DefaultQuery("mode", "2"))

	lpCfg := lp_models.LPConfig{
		Version:   version,
		Mode:      mode,
		Described: 0,
	}

	ctx := c.Request.Context()
	if ts == 0 {
		currentTS, _ := r.LPRepo.GetUserTS(ctx, userID)
		ts = currentTS
	}

	rawEvents, newTS, err := r.LPRepo.GetUpdates(ctx, userID, ts)
	if err != nil && err != redis_repo.ErrTsTooOld {
		r.Reject(c, 10, "Internal server error: "+err.Error())
		return
	}
	if err == redis_repo.ErrTsTooOld && pts == 0 {
		r.Reject(c, 907, "History outdated")
		return
	}

	history := make([]interface{}, 0)
	var legacyItems []db_models.VKApiMessageLegacy
	var modernItems []db_models.VKApiMessage

	for _, ev := range rawEvents {
		if len(history) >= eventsLimit {
			break
		}

		sliceData := ev.ToSlice(lpCfg)
		slice, ok := sliceData.([]interface{})
		if !ok || len(slice) == 0 {
			continue
		}

		history = append(history, slice)
		code := slice[0].(int)

		// Code 4: New message
		if code == 4 && (len(legacyItems) < msgsLimit && len(modernItems) < msgsLimit) {
			var msgID uint64
			switch v := slice[1].(type) {
			case uint64:
				msgID = v
			case int64:
				msgID = uint64(v)
			case int:
				msgID = uint64(v)
			}

			if maxMsgID > 0 && msgID <= maxMsgID {
				continue
			}

			var peerID int64
			switch v := slice[3].(type) {
			case int64:
				peerID = v
			case int:
				peerID = int64(v)
			case uint64:
				peerID = int64(v)
			}

			chatID := chat.GetInternalChatID(peerID, userID)

			var m db_models.Message
			q := db.Instance.Where("id = ? OR (chat_id = ? AND local_id = ?)", msgID, chatID, msgID)
			q = db_models.BuildVisibilityFilter(q, chatID, userID)
			if err := q.First(&m).Error; err == nil {
				if apiV.IsOlderThan(5, 80) {
					vkMsg := m.ToVKApiStructLegacy(db.Instance, 0, userID, peerID)
					if previewLen > 0 {
						vkMsg.Body = core.TruncateWords(vkMsg.Body, previewLen)
					}
					legacyItems = append(legacyItems, vkMsg)
				} else {
					vkMsg := m.ToVKApiStruct(db.Instance, 0, userID, peerID)
					if previewLen > 0 {
						vkMsg.Text = core.TruncateWords(vkMsg.Text, previewLen)
					}
					modernItems = append(modernItems, vkMsg)
				}
			}
		}
	}

	newPTS, _ := r.LPRepo.GetUserPTS(ctx, userID)

	var messagesResp gin.H
	var userIDs, groupIDs, chatIDs []int64

	if apiV.IsOlderThan(5, 80) {
		messagesResp = gin.H{"count": len(legacyItems), "items": legacyItems}
		core.CollectAllEntityIDsLegacy(legacyItems, &userIDs, &groupIDs, &chatIDs)
	} else {
		messagesResp = gin.H{"count": len(modernItems), "items": modernItems}
		core.CollectAllEntityIDs(modernItems, &userIDs, &groupIDs)
	}

	responsePayload := gin.H{
		"history":  history,
		"messages": messagesResp,
		"profiles": core.UniqueIDs(userIDs),
		"groups":   core.UniqueIDs(groupIDs),
		"new_pts":  newPTS,
		"new_ts":   newTS,
	}

	if len(chatIDs) > 0 {
		responsePayload["chats"] = core.UniqueIDs(chatIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": responsePayload})
}
