package db_models

import (
	"strconv"
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
	PeerID     int64  `gorm:"index" json:"peer_id"`
	InternalID string `gorm:"primaryKey;type:varchar(100)" json:"internal_id"`

	PeerType uint8  `gorm:"type:tinyint;default:0;index" json:"peer_type"`
	Title    string `gorm:"type:varchar(255)" json:"title"`

	LastMessageID uint64 `json:"last_message_id"`

	InReadID  uint64 `json:"in_read"`
	OutReadID uint64 `json:"out_read"`

	OwnerID *int64 `json:"owner_id"`

	Settings    EncryptedJSON `gorm:"type:longblob" json:"settings"`
	PinnedMsgID uint64        `json:"pinned_msg_id"`
	CreatedAt   time.Time
}

type ConversationMember struct {
	PeerID         int64  `gorm:"primaryKey;index:idx_member_lookup" json:"peer_id"`
	UserID         int64  `gorm:"primaryKey;index:idx_user_active_chats,priority:1" json:"user_id"`
	InternalChatID string `gorm:"primaryKey;index:idx_member_lookup;type:varchar(100)"`

	StartMessageID uint64 `json:"start_message_id"`
	LastReadID     uint64 `json:"last_read_id"`

	IsAdmin bool `gorm:"type:tinyint(1)" json:"is_admin"`
	IsMuted bool `gorm:"type:tinyint(1)" json:"is_muted"`

	InvitedBy int64      `json:"invited_by"`
	JoinedAt  time.Time  `gorm:"precision:3" json:"joined_at"`
	LeftAt    *time.Time `gorm:"precision:3;index:idx_user_active_chats,priority:2" json:"left_at"`
	JoinCode  *string    `gorm:"type:varchar(191)" json:"join_code"`

	LastMessageID uint64 `gorm:"index:idx_user_active_chats,priority:3"`

	Conversation Conversation `gorm:"foreignKey:InternalChatID;references:InternalID" json:"-"`
}

type ChatInvite struct {
	Code      string `gorm:"primaryKey;type:varchar(191)" json:"code"`
	PeerID    int64  `gorm:"index" json:"peer_id"`
	CreatorID int64  `json:"creator_id"`

	UsageLimit int64 `json:"usage_limit"` // 0 - unlimited
	UsageCount int64 `json:"usage_count"`

	ExpiresAt *time.Time `gorm:"precision:3" json:"expires_at"`
	CreatedAt time.Time  `gorm:"precision:3" json:"created_at"`

	Conversation Conversation `gorm:"foreignKey:PeerID;references:PeerID" json:"-"`
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
	RandomID int64  `gorm:"index" json:"random_id"` // Дедупликация

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

// ----- METHODS -----

type VKApiMessage struct {
	ID              uint64         `json:"id"`
	Date            int64          `json:"date"`
	PeerID          int64          `json:"peer_id"`
	FromID          int64          `json:"from_id"`
	Out             int            `json:"out"`
	Text            string         `json:"text"`
	RandomID        int64          `json:"random_id,omitempty"`
	Attachments     []interface{}  `json:"attachments"`
	Important       bool           `json:"important"`
	ReplyMessage    *VKApiMessage  `json:"reply_message,omitempty"`
	ForwardMessages []VKApiMessage `json:"fwd_messages,omitempty"`
	Action          interface{}    `json:"action,omitempty"`
}

func (m *Message) ToVKApiStruct(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64) VKApiMessage {
	if depth <= 0 || (m.ReplyTo == nil && m.ForwardMessages == "") {
		return m.ToVKApiStructBatch(depth, currentUserID, requestedPeerID, nil)
	}

	cache := make(map[uint64]Message)

	if m.ReplyTo != nil && *m.ReplyTo > 0 {
		var r Message
		if tx.Where("chat_id = ? AND local_id = ?", m.ChatID, *m.ReplyTo).First(&r).Error == nil {
			cache[r.LocalID] = r
		}
	}

	if m.ForwardMessages != "" {
		ids := strings.Split(m.ForwardMessages, ",")
		var fwdMsgs []Message
		tx.Where("chat_id = ? AND local_id IN ?", m.ChatID, ids).Find(&fwdMsgs)
		for _, f := range fwdMsgs {
			cache[f.LocalID] = f
		}
	}

	return m.ToVKApiStructBatch(depth, currentUserID, requestedPeerID, cache)
}

func (m *Message) ToVKApiStructBatch(depth int, currentUserID int64, requestedPeerID int64, cache map[uint64]Message) VKApiMessage {
	vkMsg := VKApiMessage{
		ID:          m.LocalID,
		Date:        m.CreatedAt.Unix(),
		PeerID:      requestedPeerID,
		FromID:      m.FromID,
		Text:        string(m.Text),
		RandomID:    m.RandomID,
		Important:   m.Important,
		Attachments: []interface{}{},
	}

	if m.FromID == currentUserID {
		vkMsg.Out = 1
	} else {
		vkMsg.Out = 0
	}

	if m.Action != "" {
		actionObj := map[string]interface{}{
			"type": m.Action,
		}

		if m.ActionMid != 0 {
			actionObj["member_id"] = m.ActionMid
		}

		if m.ActionText != "" {
			actionObj["text"] = m.ActionText
		}

		if len(m.ActionPhoto) > 0 {
			var photoData interface{}
			if err := m.ActionPhoto.Unmarshal(&photoData); err == nil {
				actionObj["photo"] = photoData
			} else {
				actionObj["photo"] = string(m.ActionPhoto)
			}
		}

		vkMsg.Action = actionObj
	}

	if m.ReplyTo != nil && *m.ReplyTo > 0 && depth > 0 && cache != nil {
		if replyMsg, ok := cache[*m.ReplyTo]; ok {
			rm := replyMsg.ToVKApiStructBatch(depth-1, currentUserID, requestedPeerID, cache)
			vkMsg.ReplyMessage = &rm
		}
	}

	if m.ForwardMessages != "" && depth > 0 && cache != nil {
		ids := strings.Split(m.ForwardMessages, ",")
		for _, idStr := range ids {
			id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
			if err != nil {
				continue
			}

			if fwdMsg, ok := cache[id]; ok {
				vkMsg.ForwardMessages = append(vkMsg.ForwardMessages, fwdMsg.ToVKApiStructBatch(depth-1, currentUserID, requestedPeerID, cache))
			}
		}
	}

	return vkMsg
}

// ---- CONVERSATIONS ----

type VKApiConversation struct {
	Peer          VKApiPeer       `json:"peer"`
	InRead        uint64          `json:"in_read"`
	OutRead       uint64          `json:"out_read"`
	UnreadCount   int             `json:"unread_count"`
	Important     bool            `json:"important,omitempty"`
	Unanswered    bool            `json:"unanswered,omitempty"`
	PushSettings  *VKPushSettings `json:"push_settings,omitempty"`
	CanWrite      VKCanWrite      `json:"can_write"`
	ChatSettings  *VKChatSettings `json:"chat_settings,omitempty"`
	LastMessageID uint64          `json:"last_message_id,omitempty"`
}

type VKApiPeer struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	LocalID int64  `json:"local_id"`
}

type VKPushSettings struct {
	DisabledUntil   int64 `json:"disabled_until,omitempty"`
	DisabledForever bool  `json:"disabled_forever,omitempty"`
	NoSound         bool  `json:"no_sound,omitempty"`
}

type VKCanWrite struct {
	Allowed bool `json:"allowed"`
	Reason  int  `json:"reason,omitempty"`
}

type VKChatSettings struct {
	MembersCount  int         `json:"members_count"`
	Title         string      `json:"title"`
	PinnedMessage interface{} `json:"pinned_message,omitempty"`
	State         string      `json:"state"`
	Photo         interface{} `json:"photo,omitempty"`
	ActiveIDs     interface{} `json:"active_ids,omitempty"`
}

func (c *Conversation) ToVKApiStruct(tx *gorm.DB, currentUserID int64, member *ConversationMember, activeIDs []int64) VKApiConversation {
	peerType := "user"
	localID := c.PeerID
	if c.PeerID > 2000000000 {
		peerType = "chat"
		localID = c.PeerID - 2000000000
	} else if c.PeerID < 0 {
		peerType = "group"
		localID = -c.PeerID
	}

	conv := VKApiConversation{
		Peer: VKApiPeer{
			ID:      c.PeerID,
			Type:    peerType,
			LocalID: localID,
		},
		LastMessageID: c.LastMessageID,
		InRead:        c.InReadID,
		OutRead:       c.OutReadID,
	}

	if member != nil {
		conv.InRead = member.LastReadID
		conv.CanWrite = VKCanWrite{
			Allowed: member.LeftAt == nil,
		}
	} else {
		conv.CanWrite = VKCanWrite{
			Allowed: false,
			Reason:  917,
		}
	}

	var unreadCount int64
	tx.Model(&Message{}).
		Where("chat_id = ? AND local_id > ? AND from_id != ?", c.InternalID, conv.InRead, currentUserID).
		Count(&unreadCount)
	conv.UnreadCount = int(unreadCount)

	if peerType == "chat" {
		settings := VKChatSettings{
			Title: c.Title,
		}

		if member == nil || member.LeftAt != nil {
			settings.State = "left"
		} else {
			settings.State = "in"
		}

		var mCount int64
		tx.Model(&ConversationMember{}).Where("peer_id = ? AND left_at IS NULL", c.PeerID).Count(&mCount)
		settings.MembersCount = int(mCount)

		if len(activeIDs) > 6 {
			settings.ActiveIDs = activeIDs[:6]
		} else {
			settings.ActiveIDs = activeIDs
		}

		if c.PinnedMsgID > 0 {
			var pMsg Message
			if err := tx.Where("chat_id = ? AND local_id = ?", c.InternalID, c.PinnedMsgID).First(&pMsg).Error; err == nil {
				res := pMsg.ToVKApiStruct(tx, 0, currentUserID, c.PeerID)
				settings.PinnedMessage = res
			}
		}

		conv.ChatSettings = &settings
	}

	return conv
}

// ---- ATTACHMENTS ----

type VKApiHistoryAttachment struct {
	MessageID  uint64      `json:"message_id"`
	Attachment interface{} `json:"attachment"` // Обьект с полями type и {media_type}
}
