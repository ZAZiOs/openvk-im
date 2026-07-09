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
	"gorm.io/gorm"
)

func Delete(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		return
	}
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	idsStr := c.Query("message_ids")
	// пока-что по умолчанию удаление происходит для всех. надо пересмотреть это потом.
	//deleteAll := c.Query("delete_for_all") == "1"

	if peerID == 0 || idsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_ids")
		return
	}

	idStrings := strings.Split(idsStr, ",")
	var localIDs []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			localIDs = append(localIDs, id)
		}
	}

	if len(localIDs) == 0 {
		r.Reject(c, 100, "Invalid message_ids format")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)
	results := make(map[string]int)

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		var msgs []db_models.Message

		if err := tx.Where("chat_id = ? AND local_id IN ?", chatID, localIDs).Find(&msgs).Error; err != nil {
			return err
		}

		now := time.Now()
		for _, msg := range msgs {
			newFlags := msg.Flags | 128

			updates := map[string]interface{}{
				"flags":      newFlags,
				"deleted_at": &now,
			}

			if err := tx.Model(&msg).Updates(updates).Error; err != nil {
				continue
			}

			results[strconv.FormatUint(msg.LocalID, 10)] = 1

			//canDeleteForAll := deleteAll && msg.FromID == currentUserID && time.Since(msg.CreatedAt).Hours() <= 24
			r.SendFlagsUpdate(currentUserID, chatID, msg.LocalID, newFlags, true)
		}

		if len(msgs) > 0 {
			chat.RefreshChatLastMessage(tx, chatID)
		}

		return nil
	})

	if err != nil {
		r.Reject(c, 10, "Internal server error during deletion")
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": results})
}

func Restore(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		return
	}
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	messageID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)

	if peerID == 0 || messageID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		var msg db_models.Message
		if err := tx.Where("chat_id = ? AND local_id = ?", chatID, messageID).First(&msg).Error; err != nil {
			return err
		}

		newFlags := msg.Flags &^ 128

		updates := map[string]interface{}{
			"flags":      newFlags,
			"deleted_at": nil,
		}

		if err := tx.Model(&msg).Updates(updates).Error; err != nil {
			return err
		}

		chat.RefreshChatLastMessage(tx, chatID)

		r.SendFlagsUpdate(currentUserID, chatID, msg.LocalID, newFlags, true)
		return nil
	})

	if err != nil {
		r.Reject(c, 910, "Can't restore this message, maybe it doesn't exist")
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
