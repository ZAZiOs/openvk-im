package db_models

import (
	"encoding/json"
	"log"
	"ovk-im/src/crypto"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Conversation struct {
	ChatID        int64          `gorm:"primaryKey;autoIncrement" json:"chat_id"`
	Type          int8           `gorm:"type:tinyint(4)" json:"type"` // 0 - p2p; 1 - community; 2 - group
	LastMessageID uint64         `json:"last_message_id"`
	OwnerID       *int64         `json:"owner_id"` // null or if group_chat user id
	Settings      datatypes.JSON `gorm:"type:json" json:"settings"`
	PinnedMsgIds  datatypes.JSON `gorm:"type:json" json:"pinned_msg_ids"`
}

type ConversationMember struct {
	ChatID     int64      `gorm:"primaryKey" json:"chat_id"`
	UserID     int64      `gorm:"primaryKey;index:idx_user_chat" json:"user_id"`
	LastReadID uint64     `json:"last_read_id"`
	IsAdmin    bool       `gorm:"type:tinyint(1)" json:"is_admin"`
	IsMuted    bool       `gorm:"type:tinyint(1)" json:"is_muted"`
	JoinedAt   time.Time  `gorm:"precision:3" json:"joined_at"`
	LeftAt     *time.Time `gorm:"precision:3" json:"left_at"`
	InvitedBy  int64      `json:"invited_by"`
	JoinCode   *string    `gorm:"type:varchar(191)" json:"join_code"`
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
	ID   uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Type int64  `json:"type"` // 0: system, 1: text

	ChatID   int64  `gorm:"index" json:"chat_id"`
	UpdateID uint64 `json:"update_id"`

	FromID  int64   `gorm:"index" json:"from_id"` // *1 - user; *-1 community;
	ReplyTo *uint64 `json:"reply_to,omitempty"`

	Text        string         `gorm:"type:text" json:"text"`
	Attachments datatypes.JSON `gorm:"type:json" json:"attachments"` // json

	Action    string `gorm:"type:varchar(50)" json:"action"` // f.e. chat_invite_user
	ActionMid int64  `json:"action_mid"`                     // f.e. userid

	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`
	EditedAt  *time.Time `gorm:"precision:3" json:"edited_at"`
	DeletedAt *time.Time `gorm:"precision:3" json:"deleted_at"`
}

type Event struct {
	ID        uint64        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64         `gorm:"index" json:"user_id"`
	Type      string        `json:"type"` // msg_new, msg_edit, chat_invite, typing
	Data      EncryptedJSON `gorm:"type:longtext" json:"data"`
	CreatedAt time.Time     `gorm:"precision:3;index;autoCreateTime" json:"created_at"`
}

// методы вынес вниз чтобы бд читать не мешали

func (m *Message) ToJSON() string {
	b, err := json.Marshal(m)
	if err != nil {
		log.Printf("ToJSON Error: %v", err)
		return "{}"
	}
	return string(b)
}

func (m *Message) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), m)
}

func (m *Message) EncryptText() {
	encrypted, err := crypto.Encrypt(m.Text)
	if err == nil {
		m.Text = encrypted
	}
}

func (m *Message) DecryptText() {
	decrypted, err := crypto.Decrypt(m.Text)
	if err == nil {
		m.Text = decrypted
	}
}

func (e *Event) ToJSON() string {
	b, err := json.Marshal(e)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (e *Event) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), e)
}

func (c *Conversation) ToJSON() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// Хуки GORM:
func (e *Event) BeforeSave(tx *gorm.DB) (err error) {
	if len(e.Data) == 0 {
		return nil
	}
	dataStr := string(e.Data)
	if strings.HasPrefix(dataStr, "{") || strings.HasPrefix(dataStr, "[") {
		encrypted, err := crypto.Encrypt(dataStr)
		if err != nil {
			return err
		}
		e.Data = EncryptedJSON(encrypted)
	}

	return nil
}

func (e *Event) AfterFind(tx *gorm.DB) (err error) {
	if len(e.Data) == 0 {
		return nil
	}

	decrypted, err := crypto.Decrypt(string(e.Data))
	if err != nil {
		return nil
	}

	e.Data = EncryptedJSON(decrypted)
	return nil
}
