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
	"gorm.io/gorm/clause"
)

func Delete(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		return
	}
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	idsStr := c.Query("message_ids")
	deleteAll := c.Query("delete_for_all") == "1"

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

	var msgs []db_models.Message
	if peerID != 0 {
		chatID := chat.GetInternalChatID(peerID, currentUserID)
		if err := dbx.Instance.Where("chat_id = ? AND (local_id IN ? OR id IN ?)", chatID, localIDs, localIDs).Find(&msgs).Error; err != nil || len(msgs) == 0 {
			r.Reject(c, 946, "Messages not found")
			return
		}
	} else {
		if err := dbx.Instance.Where("id IN ?", localIDs).Find(&msgs).Error; err != nil || len(msgs) == 0 {
			r.Reject(c, 946, "Messages not found")
			return
		}
	}

	if deleteAll {
		for _, msg := range msgs {
			if msg.FromID != currentUserID {
				r.Reject(c, 924, "Can't delete this message for all users: you are not the author")
				return
			}
			if time.Since(msg.CreatedAt).Hours() > 24 {
				r.Reject(c, 924, "Can't delete this message for all users: 24 hours have passed")
				return
			}
		}
	}

	results := make(map[string]int)

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		affectedChats := make(map[string]bool)
		for _, msg := range msgs {
			msgChatID := msg.ChatID
			affectedChats[msgChatID] = true
			if deleteAll {
				newFlags := msg.Flags | 128 | 64
				updates := map[string]interface{}{
					"flags":      newFlags,
					"deleted_at": &now,
				}

				if err := tx.Model(&msg).Updates(updates).Error; err != nil {
					continue
				}

				results[strconv.FormatUint(msg.ID, 10)] = 1
				results[strconv.FormatUint(msg.LocalID, 10)] = 1
				r.SendDeleteEvent(currentUserID, msgChatID, msg.LocalID, newFlags, true)
			} else {
				// Delete for current user only
				delRecord := db_models.DeletedMessage{
					UserID:  currentUserID,
					ChatID:  msgChatID,
					LocalID: msg.LocalID,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&delRecord).Error; err != nil {
					continue
				}

				results[strconv.FormatUint(msg.ID, 10)] = 1
				results[strconv.FormatUint(msg.LocalID, 10)] = 1
				r.SendDeleteEvent(currentUserID, msgChatID, msg.LocalID, msg.Flags|128, false)
			}
		}

		for cID := range affectedChats {
			chat.RefreshChatLastMessage(tx, cID)
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

	if messageID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: message_id")
		return
	}

	var chatID string
	if peerID != 0 {
		chatID = chat.GetInternalChatID(peerID, currentUserID)
	}

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		var msg db_models.Message
		var err error
		if chatID != "" {
			err = tx.Where("chat_id = ? AND (local_id = ? OR id = ?)", chatID, messageID, messageID).First(&msg).Error
		} else {
			err = tx.Where("id = ?", messageID).First(&msg).Error
			if err == nil {
				chatID = msg.ChatID
			}
		}
		if err != nil {
			return err
		}

		// First check if deleted only for current user
		var delRecord db_models.DeletedMessage
		res := tx.Where("user_id = ? AND chat_id = ? AND local_id = ?", currentUserID, chatID, msg.LocalID).Delete(&delRecord)
		if res.RowsAffected > 0 {
			r.SendRestoreEvent(currentUserID, chatID, msg.LocalID, msg.Flags, false)
			return nil
		}

		// Otherwise if globally deleted and current user is sender
		if msg.DeletedAt != nil {
			if msg.FromID != currentUserID {
				return gorm.ErrRecordNotFound
			}

			newFlags := msg.Flags &^ (128 | 64)
			updates := map[string]interface{}{
				"flags":      newFlags,
				"deleted_at": nil,
			}

			if err := tx.Model(&msg).Updates(updates).Error; err != nil {
				return err
			}

			chat.RefreshChatLastMessage(tx, chatID)
			r.SendRestoreEvent(currentUserID, chatID, msg.LocalID, newFlags, true)
			return nil
		}

		return gorm.ErrRecordNotFound
	})

	if err != nil {
		r.Reject(c, 910, "Can't restore this message, maybe it doesn't exist")
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
