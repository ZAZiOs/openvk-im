package entity

import (
	"ovk-im/src/db"
	models "ovk-im/src/models/db"

	"gorm.io/datatypes"
)

func CreateConversation(conv *models.Conversation) error {
	return db.Instance.Create(conv).Error
}

func GetConversationByChatID(chatID int64) (*models.Conversation, error) {
	var conv models.Conversation
	err := db.Instance.First(&conv, "chat_id = ?", chatID).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func UpdateLastMessage(chatID int64, messageID uint64) error {
	return db.Instance.Model(&models.Conversation{}).
		Where("chat_id = ?", chatID).
		Update("last_message_id", messageID).Error
}

func UpdateSettings(chatID int64, settings datatypes.JSON) error {
	return db.Instance.Model(&models.Conversation{}).
		Where("chat_id = ?", chatID).
		Update("settings", settings).Error
}

func UpdatePinnedMessages(chatID int64, pinnedIds datatypes.JSON) error {
	return db.Instance.Model(&models.Conversation{}).
		Where("chat_id = ?", chatID).
		Update("pinned_msg_ids", pinnedIds).Error
}

func ConversationExists(chatID int64) bool {
	var count int64
	db.Instance.Model(&models.Conversation{}).Where("chat_id = ?", chatID).Count(&count)
	return count > 0
}
