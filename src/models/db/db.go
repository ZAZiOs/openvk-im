package db_models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	PeerTypeUser = iota
	PeerTypeClub
	PeerTypeChat
)

type Conversation struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	InternalID string `gorm:"primaryKey;type:varchar(100)" json:"internal_id"`

	Title string `gorm:"type:varchar(255)" json:"title"`

	LastMessageID uint64 `json:"last_message_id"`

	InReadID  uint64 `json:"in_read"`
	OutReadID uint64 `json:"out_read"`

	OwnerID *int64 `json:"owner_id"`

	Settings    EncryptedJSON `gorm:"type:longblob" json:"settings"`
	PinnedMsgID uint64        `json:"pinned_msg_id"`
	CreatedAt   time.Time
}

type ConversationMember struct {
	UserID         int64  `gorm:"primaryKey;index:idx_user_active_chats,priority:1" json:"user_id"`
	InternalChatID string `gorm:"primaryKey;index:idx_member_lookup;type:varchar(100)"`

	StartMessageID  uint64 `json:"start_message_id"`
	LastReadID      uint64 `json:"last_read_id"`
	DeletedBeforeID uint64 `json:"deleted_before_id"`

	IsAdmin bool  `gorm:"type:tinyint(1)" json:"is_admin"`
	IsMuted bool  `gorm:"type:tinyint(1)" json:"is_muted"`
	Flags   int64 `json:"flags"`

	InvitedBy int64      `json:"invited_by"`
	JoinedAt  time.Time  `gorm:"precision:3" json:"joined_at"`
	LeftAt    *time.Time `gorm:"precision:3;index:idx_user_active_chats,priority:2" json:"left_at"`
	JoinCode  *string    `gorm:"type:varchar(191)" json:"join_code"`

	LastMessageID uint64 `gorm:"index:idx_user_active_chats,priority:3"`

	Conversation Conversation `gorm:"foreignKey:InternalChatID;references:InternalID" json:"-"`
}

type ChatInvite struct {
	Code           string `gorm:"primaryKey;type:varchar(191)" json:"code"`
	InternalChatID int64  `gorm:"index:idx_member_lookup;type:varchar(100)"`
	CreatorID      int64  `json:"creator_id"`

	UsageLimit int64 `json:"usage_limit"` // 0 - unlimited
	UsageCount int64 `json:"usage_count"`

	ExpiresAt *time.Time `gorm:"precision:3" json:"expires_at"`
	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`

	Conversation Conversation `gorm:"foreignKey:InternalChatID;references:InternalID" json:"-"`
}

/*
Actions:
chat_photo_update  -> photo: {photo_50, photo_100, photo_200}
chat_photo_remove
chat_create        -> text = "chatName"
chat_title_update  -> text = "chatName"
chat_invite_user   -> member_id = id
chat_kick_user     -> member_id = id
chat_pin_message   -> member_id = id // кто совершил действие
chat_unpin_message -> member_id = id // кто совершил действие
chat_invite_user_by_link
*/

type Message struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ChatID   string `gorm:"index:idx_chat_local,priority:1;type:varchar(100)" json:"chat_id"`
	LocalID  uint64 `gorm:"index:idx_chat_local,priority:2" json:"local_id"`
	RandomID int64  `gorm:"index" json:"random_id"`

	FromID  int64   `gorm:"index" json:"from_id"` // *1 - user; *-1 community;
	ReplyTo *uint64 `json:"reply_to,omitempty"`

	Text            EncryptedJSON `gorm:"type:longblob" json:"text"`
	Attachments     EncryptedJSON `gorm:"type:longblob" json:"attachments"`
	ForwardMessages string        `gorm:"type:text"`

	Action      string        `gorm:"type:varchar(50)" json:"action"` // f.e. chat_invite_user
	ActionMid   int64         `json:"action_mid"`                     // f.e. userid
	ActionText  string        `json:"action_text"`
	ActionPhoto EncryptedJSON `gorm:"type:longblob" json:"action_photo"`

	Important bool `gorm:"type:tinyint(1)" json:"important"`

	Flags     uint64     `json:"flags"`
	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`
	EditedAt  *time.Time `gorm:"precision:3" json:"edited_at"`
	DeletedAt *time.Time `gorm:"precision:3" json:"deleted_at"`

	Conversation Conversation `gorm:"foreignKey:ChatID;references:InternalID" json:"-"`
}

type MessageSearchIndex struct {
	MessageID uint64 `gorm:"primaryKey;index:idx_search_word"`
	ChatID    string `gorm:"index:idx_search_word;type:varchar(100)"`
	WordHash  []byte `gorm:"primaryKey;type:binary(32);index:idx_search_word"`
}

type ImportantMessage struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int64     `gorm:"uniqueIndex:idx_user_msg" json:"user_id"`
	MessageID uint64    `gorm:"uniqueIndex:idx_user_msg" json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

type ImState struct {
	UserID int64  `gorm:"primaryKey;autoIncrement:false"`
	PTS    uint64 `gorm:"default:1"`
}

type ConversationMemberPeriod struct {
	ID             uint64  `gorm:"primaryKey;autoIncrement"`
	InternalChatID string  `gorm:"index:idx_period_lookup;type:varchar(100)"`
	UserID         int64   `gorm:"index:idx_period_lookup"`
	StartLocalID   uint64  `json:"start_local_id"`
	EndLocalID     *uint64 `json:"end_local_id"` // NULL = still active
}

type DeletedMessage struct {
	UserID  int64  `gorm:"primaryKey"`
	ChatID  string `gorm:"primaryKey;type:varchar(100)"`
	LocalID uint64 `gorm:"primaryKey"`
}

// BuildVisibilityFilter applies visibility filters to a gorm query for messages.
// It filters out:
// 1. Messages globally deleted (deleted_at IS NOT NULL)
// 2. Messages outside the user's presence periods (conversation_member_periods)
// 3. Messages individually deleted by the user (deleted_messages)
// 4. Messages before the user's DeletedBeforeID (from deleteConversation)
func BuildVisibilityFilter(query *gorm.DB, chatID string, userID int64) *gorm.DB {
	if userID == 0 {
		return query
	}

	query = query.Where("messages.deleted_at IS NULL")

	if chatID != "" {
		if strings.HasPrefix(chatID, "c") {
			query = query.Where(
				"(NOT EXISTS (SELECT 1 FROM conversation_member_periods p0 WHERE p0.internal_chat_id = ?) OR EXISTS (SELECT 1 FROM conversation_member_periods p WHERE p.internal_chat_id = messages.chat_id AND p.user_id = ? AND messages.local_id >= p.start_local_id AND (p.end_local_id IS NULL OR messages.local_id <= p.end_local_id)))",
				chatID, userID,
			)
		}
	} else {
		query = query.Where(
			"(messages.chat_id NOT LIKE 'c%' OR NOT EXISTS (SELECT 1 FROM conversation_member_periods p0 WHERE p0.internal_chat_id = messages.chat_id) OR EXISTS (SELECT 1 FROM conversation_member_periods p WHERE p.internal_chat_id = messages.chat_id AND p.user_id = ? AND messages.local_id >= p.start_local_id AND (p.end_local_id IS NULL OR messages.local_id <= p.end_local_id)))",
			userID,
		)
	}

	// Filter out individually deleted messages
	query = query.Where(
		"NOT EXISTS (SELECT 1 FROM deleted_messages dm WHERE dm.chat_id = messages.chat_id AND dm.user_id = ? AND dm.local_id = messages.local_id)",
		userID,
	)

	// Filter out messages before DeletedBeforeID
	query = query.Where(
		"messages.local_id > COALESCE((SELECT deleted_before_id FROM conversation_members WHERE internal_chat_id = messages.chat_id AND user_id = ?), 0)",
		userID,
	)

	return query
}
