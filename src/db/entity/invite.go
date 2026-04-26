package entity

import (
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
	"time"

	"gorm.io/gorm"
)

func CreateInvite(invite *models.ChatInvite) error {
	if invite.CreatedAt.IsZero() {
		invite.CreatedAt = time.Now()
	}
	return db.Instance.Create(invite).Error
}

func GetInviteByCode(code string) (*models.ChatInvite, error) {
	var invite models.ChatInvite
	err := db.Instance.Where("code = ? AND (expires_at IS NULL OR expires_at > ?)", code, time.Now()).
		First(&invite).Error

	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func DeleteInvite(code string) error {
	return db.Instance.Delete(&models.ChatInvite{}, "code = ?", code).Error
}

func GetInvitesByChatID(chatID int64) ([]models.ChatInvite, error) {
	var invites []models.ChatInvite
	err := db.Instance.Where("chat_id = ?", chatID).Find(&invites).Error
	return invites, err
}

func IncrementUsage(code string) error {
	return db.Instance.Model(&models.ChatInvite{}).
		Where("code = ?", code).
		Update("usage_count", gorm.Expr("usage_count + 1")).Error
}

func CanUseInvite(invite *models.ChatInvite) bool {
	if invite.UsageLimit == 0 {
		return true
	}
	return invite.UsageCount < invite.UsageLimit
}
