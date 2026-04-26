package entity

import (
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
	"time"
)

func AddMember(member *models.ConversationMember) error {
	return db.Instance.Save(member).Error
}

func GetMember(chatID int64, userID int64) (*models.ConversationMember, error) {
	var member models.ConversationMember
	err := db.Instance.Where("chat_id = ? AND user_id = ?", chatID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func UpdateLastRead(chatID int64, userID int64, messageID uint64) error {
	return db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("last_read", messageID).Error
}

func SetMute(chatID int64, userID int64, muted bool) error {
	return db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("is_muted", muted).Error
}

func LeaveMember(chatID int64, userID int64) error {
	return db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("left_at", time.Now()).Error
}

func IsAdmin(chatID int64, userID int64) bool {
	var member models.ConversationMember
	err := db.Instance.Select("is_admin").
		Where("chat_id = ? AND user_id = ? AND left_at IS NULL", chatID, userID).
		First(&member).Error
	return err == nil && member.IsAdmin
}

func GetActiveMembersIDs(chatID int64) ([]int64, error) {
	var ids []int64
	err := db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND left_at IS NULL", chatID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func IsMember(chatID int64, userID int64) (bool, error) {
	var count int64
	err := db.Instance.Model(&models.ConversationMember{}).
		Where("chat_id = ? AND user_id = ? AND left_at IS NULL", chatID, userID).
		Count(&count).Error
	return count > 0, err
}
