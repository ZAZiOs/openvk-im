package messages

import (
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
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
	var globalIDs []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			globalIDs = append(globalIDs, id)
		}
	}

	results := make(map[string]int)

	for _, gID := range globalIDs {
		var msg db_models.Message
		if err := dbx.Instance.Where("id = ?", gID).First(&msg).Error; err != nil {
			continue
		}
		canAccess, _ := chat.IsUserInChat(dbx.Instance, msg.ChatID, currentUserID)
		if !canAccess {
			continue
		}
		msgPeerID := chat.DerivePeerID(msg.ChatID, currentUserID)

		canDeleteForAll := deleteAll && msg.FromID == currentUserID && time.Since(msg.CreatedAt).Hours() <= 24

		newFlags := msg.Flags | 128
		if err := dbx.Instance.Model(&msg).Update("flags", newFlags).Error; err != nil {
			continue
		}

		results[strconv.FormatUint(gID, 10)] = 1

		r.SendFlagsUpdate(currentUserID, msgPeerID, msg.LocalID, newFlags, canDeleteForAll)
	}

	c.JSON(http.StatusOK, gin.H{"response": results})
}

func Restore(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	gID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)

	var msg db_models.Message
	if err := dbx.Instance.Where("id = ?", gID).First(&msg).Error; err != nil {
		r.Reject(c, 15, "Access denied: message not found")
		return
	}

	canAccess, _ := chat.IsUserInChat(dbx.Instance, msg.ChatID, currentUserID)
	if !canAccess {
		r.Reject(c, 15, "Access denied")
		return
	}

	newFlags := msg.Flags &^ 128

	if err := dbx.Instance.Model(&msg).Update("flags", newFlags).Error; err != nil {
		r.Reject(c, 10, "Internal error during restore")
		return
	}

	msgPeerID := chat.DerivePeerID(msg.ChatID, currentUserID)

	r.SendFlagsUpdate(currentUserID, msgPeerID, msg.LocalID, newFlags, false)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
