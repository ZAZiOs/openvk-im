package entity

import (
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
	"time"
)

func CreateMessage(msg *models.Message) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	return db.Instance.Create(msg).Error
}

func GetMessageByID(id uint64) (*models.Message, error) {
	var msg models.Message
	err := db.Instance.First(&msg, id).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func UpdateMessage(msg *models.Message) error {
	msg.EditedAt = func(t time.Time) *time.Time { return &t }(time.Now())
	return db.Instance.Save(msg).Error
}
func DeleteMessage(id uint64) error {
	return db.Instance.Model(&models.Message{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

func RestoreMessage(id uint64) error {
	return db.Instance.Model(&models.Message{}).
		Where("id = ?", id).
		Update("deleted_at", nil).Error
}

func GetLastUpdateID(chatID int64) (uint64, error) {
	var msg models.Message
	err := db.Instance.Select("update_id").
		Where("chat_id = ?", chatID).
		Order("update_id DESC").
		First(&msg).Error
	if err != nil {
		return 0, err
	}
	return msg.UpdateID, nil
}
