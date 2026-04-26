package repo

import (
	"encoding/json"
	"errors"
	"ovk-im/src/db"
	"ovk-im/src/db/entity"
	models "ovk-im/src/models/db"
	rdm "ovk-im/src/models/redis"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AcceptMsg обрабатывает входящее сообщение, проверяет права и создает записи в БД
func AcceptMsg(data rdm.SendMsgPayload) ([]*models.Message, error) {
	// Проверяем, состоит ли пользователь в чате
	if data.MsgType != 0 || data.FromID != 0 {
		isMember, err := entity.IsMember(data.ChatID, data.FromID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, errors.New("user_not_in_chat")
		}
	}

	var createdMessages []*models.Message
	attachmentsJSON, _ := json.Marshal(data.Attachments)

	err := db.Instance.Transaction(func(tx *gorm.DB) error {
		iterations := 1
		if data.MsgType == 0 && len(data.ActionMids) > 0 {
			iterations = len(data.ActionMids)
		}

		for i := 0; i < iterations; i++ {
			var lastUpdateID uint64
			tx.Model(&models.Message{}).
				Where("chat_id = ?", data.ChatID).
				Select("COALESCE(MAX(update_id), 0)").
				Row().Scan(&lastUpdateID)

			var replyToPtr *uint64
			if data.ReplyTo > 0 {
				val := data.ReplyTo
				replyToPtr = &val
			}

			msg := &models.Message{
				Type:        int64(data.MsgType),
				ChatID:      data.ChatID,
				UpdateID:    lastUpdateID + 1,
				FromID:      data.FromID,
				ReplyTo:     replyToPtr,
				Text:        data.Text,
				Attachments: datatypes.JSON(attachmentsJSON),
				Action:      data.Action,
				CreatedAt:   time.Now(),
			}

			if data.MsgType == 0 && len(data.ActionMids) > 0 {
				msg.ActionMid = data.ActionMids[i]
			}

			if err := tx.Create(msg).Error; err != nil {
				return err
			}

			createdMessages = append(createdMessages, msg)

			if i == iterations-1 {
				err := tx.Model(&models.Conversation{}).
					Where("chat_id = ?", data.ChatID).
					Update("last_message_id", msg.ID).Error
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	return createdMessages, err
}
