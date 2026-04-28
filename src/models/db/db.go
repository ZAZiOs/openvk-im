package db_models

import (
	"time"
)

type Conversation struct {
	ID            uint64        `gorm:"primaryKey;autoIncrement" json:"-"`
	PeerID        int64         `gorm:"uniqueIndex" json:"peer_id"`
	LastMessageID uint64        `json:"last_message_id"`
	InReadID      uint64        `json:"in_read"`
	OutReadID     uint64        `json:"out_read"`
	OwnerID       *int64        `json:"owner_id"`
	Settings      EncryptedJSON `gorm:"type:longblob" json:"settings"`
	PinnedMsgID   uint64        `json:"pinned_msg_id"`
}

type ConversationMember struct {
	PeerID int64 `gorm:"primaryKey;index:idx_member" json:"peer_id"`
	UserID int64 `gorm:"primaryKey;index:idx_member" json:"user_id"`

	LastReadID uint64 `json:"last_read_id"`

	IsAdmin bool `gorm:"type:tinyint(1)" json:"is_admin"`
	IsMuted bool `gorm:"type:tinyint(1)" json:"is_muted"`

	InvitedBy int64      `json:"invited_by"`
	JoinedAt  time.Time  `gorm:"precision:3" json:"joined_at"`
	LeftAt    *time.Time `gorm:"precision:3" json:"left_at"`
	JoinCode  *string    `gorm:"type:varchar(191)" json:"join_code"`
}

type ChatInvite struct {
	Code      string `gorm:"primaryKey;type:varchar(191)" json:"code"`
	ChatID    int64  `gorm:"index" json:"chat_id"`
	CreatorID int64  `json:"creator_id"`

	UsageLimit int64 `json:"usage_limit"` // 0 - unlimited
	UsageCount int64 `json:"usage_count"`

	ExpiresAt *time.Time `gorm:"precision:3" json:"expires_at"`
	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`
}

type Message struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatID  int64  `gorm:"index" json:"chat_id"`
	LocalID uint64 `json:"local_id"`

	FromID  int64   `gorm:"index" json:"from_id"` // *1 - user; *-1 community;
	ReplyTo *uint64 `json:"reply_to,omitempty"`

	Text        EncryptedJSON `gorm:"type:longblob" json:"text"`
	Attachments EncryptedJSON `gorm:"type:longblob" json:"attachments"`

	Action    string `gorm:"type:varchar(50)" json:"action"` // f.e. chat_invite_user
	ActionMid int64  `json:"action_mid"`                     // f.e. userid

	Flags     uint64     `json:"flags"`
	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`
	EditedAt  *time.Time `gorm:"precision:3" json:"edited_at"`
	DeletedAt *time.Time `gorm:"precision:3" json:"deleted_at"`
}

type ImState struct {
	UserID int64  `gorm:"primaryKey;autoIncrement:false"`
	PTS    uint64 `gorm:"default:1"`
}
