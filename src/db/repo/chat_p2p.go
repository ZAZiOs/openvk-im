package repo

import (
	"errors"
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
	"time"

	"gorm.io/gorm"
)

// CreateP2PConversation создает диалог между двумя пользователями, если его нет
func CreateP2PConversation(userID1, userID2 int64) (*models.Conversation, error) {
	var conv models.Conversation

	err := db.Instance.Transaction(func(tx *gorm.DB) error {
		/* Проверяем что не существует диалога типа 0 (p2p)
		в котором участвуют два переданных ConversationMember.
		Если есть - значит этот чат уже существует.*/
		err := tx.Table("conversations").
			Select("conversations.*").
			Joins("JOIN conversation_members cm1 ON cm1.chat_id = conversations.chat_id").
			Joins("JOIN conversation_members cm2 ON cm2.chat_id = conversations.chat_id").
			Where("conversations.type = ?", 0).
			Where("cm1.user_id = ?", userID1).
			Where("cm2.user_id = ?", userID2).
			Where("cm1.left_at IS NULL AND cm2.left_at IS NULL").
			First(&conv).Error

		if err == nil {
			return nil
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		conv = models.Conversation{
			Type:    0, // P2P
			OwnerID: nil,
		}

		if err := tx.Create(&conv).Error; err != nil {
			return err
		}

		member1 := models.ConversationMember{
			ChatID:   conv.ChatID,
			UserID:   userID1,
			JoinedAt: time.Now(),
		}

		member2 := models.ConversationMember{
			ChatID:   conv.ChatID,
			UserID:   userID2,
			JoinedAt: time.Now(),
		}

		if err := tx.Create(&member1).Error; err != nil {
			return err
		}
		if err := tx.Create(&member2).Error; err != nil {
			return err
		}

		return nil
	})

	return &conv, err
}
