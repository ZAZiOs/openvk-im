package messages

import (
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Delete(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	idsStr := c.Query("message_ids")
	deleteAll := c.Query("delete_for_all") == "1"

	if idsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: message_ids")
		return
	}

	idStrings := strings.Split(idsStr, ",")
	var ids []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	results := make(map[string]int)

	for _, msgLocalID := range ids {
		var msg db_models.Message
		if err := dbx.Instance.Where("local_id = ? AND (from_id = ? OR chat_id = ?)", msgLocalID, currentUserID, currentUserID).First(&msg).Error; err != nil {
			continue
		}

		canDeleteForAll := deleteAll && time.Since(msg.CreatedAt).Hours() <= 24

		newFlags := msg.Flags | 128

		if err := dbx.Instance.Model(&msg).Update("flags", newFlags).Error; err != nil {
			continue
		}

		results[strconv.FormatUint(msgLocalID, 10)] = 1

		r.SendFlagsUpdate(currentUserID, msg.PeerID, msg.LocalID, newFlags, canDeleteForAll)
	}

	c.JSON(http.StatusOK, gin.H{"response": results})
}

func Restore(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	msgID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)

	var msg db_models.Message
	err := dbx.Instance.Where("local_id = ? AND from_id = ?", msgID, currentUserID).First(&msg).Error
	if err != nil {
		r.Reject(c, 15, "Access denied: message not found")
		return
	}

	newFlags := msg.Flags &^ 128

	if err := dbx.Instance.Model(&msg).Update("flags", newFlags).Error; err != nil {
		r.Reject(c, 10, "Internal error during restore")
		return
	}

	r.SendFlagsUpdate(currentUserID, msg.PeerID, msg.LocalID, newFlags, false)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
