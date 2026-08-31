package db_models

import (
	"strconv"
	"strings"

	"ovk-im/src/db"

	"gorm.io/gorm"
)

// VKApiMessage represents the modern VK API message object (>= 5.80).
type VKApiMessage struct {
	ID                    uint64         `json:"id"`
	ConversationMessageID uint64         `json:"conversation_message_id"`
	GlobalID              uint64         `json:"global_id,omitempty"`
	Date                  int64          `json:"date"`
	IsEdited              bool           `json:"edited,omitempty"`
	EditedAt              int64          `json:"edited_at,omitempty"`
	PeerID                int64          `json:"peer_id"`
	FromID                int64          `json:"from_id"`
	Out                   int            `json:"out"`
	ReadState             int            `json:"read_state"`
	ReadBy                []int64        `json:"read_by,omitempty"`
	Text                  string         `json:"text"`
	RandomID              int64          `json:"random_id,omitempty"`
	Attachments           string         `json:"attachments"`
	Important             bool           `json:"important"`
	IsPinned              int            `json:"is_pinned,omitempty"`
	ReplyMessage          *VKApiMessage  `json:"reply_message,omitempty"`
	ForwardMessages       []VKApiMessage `json:"fwd_messages,omitempty"`
	Action                interface{}    `json:"action,omitempty"`
}

// VKApiMessageLegacy represents the legacy VK API message format for versions < 5.80 (e.g. 5.20).
type VKApiMessageLegacy struct {
	ID              uint64               `json:"id"`
	Date            int64                `json:"date"`
	Out             int                  `json:"out"`
	UserID          int64                `json:"user_id"`
	FromID          int64                `json:"from_id,omitempty"`
	ReadState       int                  `json:"read_state"`
	Title           string               `json:"title,omitempty"`
	Body            string               `json:"body"`
	Attachments     string               `json:"attachments"`
	ForwardMessages []VKApiMessageLegacy `json:"fwd_messages,omitempty"`
	Important       bool                 `json:"important,omitempty"`
	Deleted         int                  `json:"deleted"`
	Emoji           int                  `json:"emoji"`
	ChatID          int64                `json:"chat_id,omitempty"`
	ChatActive      []int64              `json:"chat_active,omitempty"`
	UsersCount      int                  `json:"users_count,omitempty"`
	AdminID         int64                `json:"admin_id,omitempty"`
	Action          interface{}          `json:"action,omitempty"`
	ActionMid       int64                `json:"action_mid,omitempty"`
	ActionEmail     string               `json:"action_email,omitempty"`
	ActionText      string               `json:"action_text,omitempty"`
	Photo50         string               `json:"photo_50,omitempty"`
	Photo100        string               `json:"photo_100,omitempty"`
	Photo200        string               `json:"photo_200,omitempty"`
}

type MemberReadState struct {
	UserID     int64  `gorm:"column:user_id"`
	LastReadID uint64 `gorm:"column:last_read_id"`
}

func GetMessageReadInfo(tx *gorm.DB, chatID string, localID uint64, fromID int64, currentUserID int64, readCache map[string][]MemberReadState) (int, []int64) {
	var members []MemberReadState
	if readCache != nil {
		members = readCache[chatID]
	}
	if members == nil && chatID != "" {
		dbRef := tx
		if dbRef == nil {
			dbRef = db.Instance
		}
		if dbRef != nil {
			dbRef.Table("conversation_members").
				Select("user_id, last_read_id").
				Where("internal_chat_id = ?", chatID).
				Scan(&members)
			if readCache != nil {
				readCache[chatID] = members
			}
		}
	}

	readBy := make([]int64, 0)
	for _, mbr := range members {
		if mbr.LastReadID >= localID {
			readBy = append(readBy, mbr.UserID)
		}
	}

	readState := 0
	if fromID == currentUserID {
		hasOtherMember := false
		for _, mbr := range members {
			if mbr.UserID != currentUserID {
				hasOtherMember = true
				if mbr.LastReadID >= localID {
					readState = 1
					break
				}
			}
		}
		if !hasOtherMember {
			for _, mbr := range members {
				if mbr.LastReadID >= localID {
					readState = 1
					break
				}
			}
		}
	} else {
		for _, mbr := range members {
			if mbr.UserID == currentUserID && mbr.LastReadID >= localID {
				readState = 1
				break
			}
		}
	}

	return readState, readBy
}

func (m *Message) ToVKApiStruct(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64) VKApiMessage {
	if depth <= 0 || (m.ReplyTo == nil && m.ForwardMessages == "") {
		return m.ToVKApiStructBatch(tx, depth, currentUserID, requestedPeerID, nil, nil, nil)
	}

	cache := make(map[uint64]Message)

	if tx != nil {
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
	}

	return m.ToVKApiStructBatch(tx, depth, currentUserID, requestedPeerID, cache, nil, nil)
}

func (m *Message) ToVKApiStructBatch(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64, cache map[uint64]Message, readCache map[string][]MemberReadState, pinnedCache map[string]uint64) VKApiMessage {
	vkMsg := VKApiMessage{
		ID:                    m.ID,
		ConversationMessageID: m.LocalID,
		GlobalID:              m.ID,
		Date:                  m.CreatedAt.Unix(),
		PeerID:                requestedPeerID,
		FromID:                m.FromID,
		Text:                  string(m.Text),
		RandomID:              m.RandomID,
		Important:             m.Important,
		Attachments:           string(m.Attachments),
	}

	if m.FromID == currentUserID {
		vkMsg.Out = 1
	} else {
		vkMsg.Out = 0
	}

	vkMsg.ReadState, vkMsg.ReadBy = GetMessageReadInfo(tx, m.ChatID, m.LocalID, m.FromID, currentUserID, readCache)

	var pinnedMsgID uint64
	if pinnedCache != nil {
		if pid, ok := pinnedCache[m.ChatID]; ok {
			pinnedMsgID = pid
		} else if tx != nil && m.ChatID != "" {
			tx.Table("conversations").Select("pinned_msg_id").Where("internal_id = ?", m.ChatID).Scan(&pinnedMsgID)
			pinnedCache[m.ChatID] = pinnedMsgID
		}
	} else if m.Conversation.PinnedMsgID > 0 {
		pinnedMsgID = m.Conversation.PinnedMsgID
	} else if tx != nil && m.ChatID != "" {
		tx.Table("conversations").Select("pinned_msg_id").Where("internal_id = ?", m.ChatID).Scan(&pinnedMsgID)
	}

	if pinnedMsgID > 0 && m.LocalID == pinnedMsgID {
		vkMsg.IsPinned = 1
	} else {
		vkMsg.IsPinned = 0
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
			rm := replyMsg.ToVKApiStructBatch(tx, depth-1, currentUserID, requestedPeerID, cache, readCache, pinnedCache)
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
				vkMsg.ForwardMessages = append(vkMsg.ForwardMessages, fwdMsg.ToVKApiStructBatch(tx, depth-1, currentUserID, requestedPeerID, cache, readCache, pinnedCache))
			}
		}
	}

	if m.EditedAt != nil {
		vkMsg.IsEdited = true
		vkMsg.EditedAt = m.EditedAt.Unix()
	} else {
		vkMsg.IsEdited = false
	}

	return vkMsg
}

func (m *Message) ToVKApiStructVersioned(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64, apiV ApiV) any {
	if apiV.IsOlderThan(5, 80) {
		return m.ToVKApiStructLegacy(tx, depth, currentUserID, requestedPeerID)
	}
	return m.ToVKApiStruct(tx, depth, currentUserID, requestedPeerID)
}

func (m *Message) ToVKApiStructBatchVersioned(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64, cache map[uint64]Message, readCache map[string][]MemberReadState, pinnedCache map[string]uint64, apiV ApiV) any {
	if apiV.IsOlderThan(5, 80) {
		return m.ToVKApiStructBatchLegacy(tx, depth, currentUserID, requestedPeerID, cache, readCache, pinnedCache)
	}
	return m.ToVKApiStructBatch(tx, depth, currentUserID, requestedPeerID, cache, readCache, pinnedCache)
}

func (m *Message) ToVKApiStructLegacy(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64) VKApiMessageLegacy {
	if depth <= 0 || m.ForwardMessages == "" {
		return m.ToVKApiStructBatchLegacy(tx, depth, currentUserID, requestedPeerID, nil, nil, nil)
	}

	cache := make(map[uint64]Message)
	if tx != nil && m.ForwardMessages != "" {
		ids := strings.Split(m.ForwardMessages, ",")
		var fwdMsgs []Message
		tx.Where("chat_id = ? AND local_id IN ?", m.ChatID, ids).Find(&fwdMsgs)
		for _, f := range fwdMsgs {
			cache[f.LocalID] = f
		}
	}

	return m.ToVKApiStructBatchLegacy(tx, depth, currentUserID, requestedPeerID, cache, nil, nil)
}

func (m *Message) ToVKApiStructBatchLegacy(tx *gorm.DB, depth int, currentUserID int64, requestedPeerID int64, cache map[uint64]Message, readCache map[string][]MemberReadState, pinnedCache map[string]uint64) VKApiMessageLegacy {
	userID := m.FromID
	if m.FromID == currentUserID {
		if requestedPeerID > 0 && requestedPeerID < 2000000000 {
			userID = requestedPeerID
		}
	}

	hasEmoji := 0
	for _, r := range string(m.Text) {
		if r > 0x2000 {
			hasEmoji = 1
			break
		}
	}

	vkMsg := VKApiMessageLegacy{
		ID:          m.ID,
		Date:        m.CreatedAt.Unix(),
		UserID:      userID,
		FromID:      m.FromID,
		Body:        string(m.Text),
		Attachments: string(m.Attachments),
		Important:   m.Important,
		Emoji:       hasEmoji,
		Deleted:     0,
	}

	if m.FromID == currentUserID {
		vkMsg.Out = 1
	} else {
		vkMsg.Out = 0
	}

	vkMsg.ReadState, _ = GetMessageReadInfo(tx, m.ChatID, m.LocalID, m.FromID, currentUserID, readCache)

	if requestedPeerID > 2000000000 {
		vkMsg.ChatID = requestedPeerID - 2000000000
		vkMsg.Title = m.Conversation.Title
		if m.Conversation.OwnerID != nil {
			vkMsg.AdminID = *m.Conversation.OwnerID
		}
	}

	if m.Action != "" {
		vkMsg.Action = m.Action
		vkMsg.ActionMid = m.ActionMid
		vkMsg.ActionText = m.ActionText
	}

	if m.ForwardMessages != "" && depth > 0 && cache != nil {
		ids := strings.Split(m.ForwardMessages, ",")
		for _, idStr := range ids {
			id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
			if err != nil {
				continue
			}

			if fwdMsg, ok := cache[id]; ok {
				vkMsg.ForwardMessages = append(vkMsg.ForwardMessages, fwdMsg.ToVKApiStructBatchLegacy(tx, depth-1, currentUserID, requestedPeerID, cache, readCache, pinnedCache))
			}
		}
	}

	return vkMsg
}
