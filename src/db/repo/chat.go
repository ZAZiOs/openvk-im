package repo

import (
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
)

// GetChatMemberIDs возвращает список ID всех участников чата.
func GetChatMemberIDs(chatID int64) ([]int64, error) {
	var ids []int64

	err := db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND left_at IS NULL", chatID).
		Pluck("user_id", &ids).Error

	if err != nil {
		return nil, err
	}

	return ids, nil
}
