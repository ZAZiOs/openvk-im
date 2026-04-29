package messages

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func MarkAsImportant(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	idsStr := c.Query("message_ids")
	important, _ := strconv.Atoi(c.DefaultQuery("important", "1"))

	if idsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: message_ids")
		return
	}

	parts := strings.Split(idsStr, ",")
	var ids []uint64
	for _, p := range parts {
		if id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	type MsgInfo struct {
		ID     uint64
		PeerID int64
	}

	var results []MsgInfo
	db.Instance.Table("messages").
		Select("messages.id, conversations.peer_id").
		Joins("left join conversations on conversations.peer_id = messages.peer_id").
		Where("messages.id IN ?", ids).
		Scan(&results)

	for _, res := range results {
		if important == 1 {
			db.Instance.Clauses(clause.OnConflict{DoNothing: true}).Create(&db_models.ImportantMessage{
				UserID:    currentUserID,
				MessageID: res.ID,
				CreatedAt: time.Now(),
			})
			r.LPRepo.PushEvent(c.Request.Context(), currentUserID, "msg_set_flags", lp_models.MsgSetFlagsEvent{
				MessageID: res.ID,
				Mask:      lp_models.MessageFlags{Value: lp_models.FlagImportant},
				PeerID:    res.PeerID,
			})
		} else {
			db.Instance.Where("user_id = ? AND message_id = ?", currentUserID, res.ID).Delete(&db_models.ImportantMessage{})
			r.LPRepo.PushEvent(c.Request.Context(), currentUserID, "msg_reset_flags", lp_models.MsgResetFlagsEvent{
				MessageID: res.ID,
				Mask:      lp_models.MessageFlags{Value: lp_models.FlagImportant},
				PeerID:    res.PeerID,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"response": ids})
}

func GetImportantMessages(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	extended := c.Query("extended") == "1"

	var msgs []db_models.Message

	query := db.Instance.Table("messages").
		Joins("JOIN important_messages ON important_messages.message_id = messages.id").
		Where("important_messages.user_id = ?", currentUserID)

	var totalCount int64
	query.Count(&totalCount)

	err := query.Order("important_messages.created_at DESC").
		Limit(count).Offset(offset).
		Find(&msgs).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	responseItems := make([]db_models.VKApiMessage, len(msgs))
	userIDsMap := make(map[int64]bool)

	for i, m := range msgs {
		v := m.ToVKApiStruct(db.Instance, 0, currentUserID)
		v.Important = true
		responseItems[i] = v

		if extended && m.FromID > 0 {
			userIDsMap[m.FromID] = true
		}
	}

	result := gin.H{
		"count": totalCount,
		"items": responseItems,
	}

	if extended {
		uIDs := []int64{}
		for id := range userIDsMap {
			uIDs = append(uIDs, id)
		}
		result["profiles"] = uIDs
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}
