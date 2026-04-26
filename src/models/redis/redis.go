package redis_models

import "encoding/json"

type Envelope struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// ------- MESSAGES -------
type MsgAttachment struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
}

type SendMsgPayload struct {
	ChatID      int64           `json:"chat_id"`
	FromID      int64           `json:"from_id"`  // *-1 community
	MsgType     int             `json:"msg_type"` // 0: system, 1: text
	Text        string          `json:"text"`
	Attachments []MsgAttachment `json:"attachments"`
	ReplyTo     uint64          `json:"reply_to"`

	// system (MsgType = 0)
	Action     string  `json:"action,omitempty"`      // f.e. "chat_invite_users"
	ActionMids []int64 `json:"action_mids,omitempty"` // f.e. [1,2,3] will make 3 system msgs
}

type EditMsgPayload struct {
	MsgID       int64           `json:"msg_id"`
	ChatID      int64           `json:"chat_id"`
	Text        string          `json:"text"`
	Attachments []MsgAttachment `json:"attachments"`
}

// delete, restore, pin, unpin, markAsRead
type MsgOperatorPayload struct {
	OperatorID int64 `json:"operator_id"`
	ChatID     int64 `json:"chat_id"`
	MsgID      int64 `json:"msg_id"`
}

// ------- CHATS -------

type CreateChatPayload struct {
	Title     string  `json:"title"`
	CreatorID int64   `json:"creator_id"`
	UserIDs   []int64 `json:"user_ids"`
}

type ChatUserPayload struct {
	ChatID     int64 `json:"chat_id"`
	UserID     int64 `json:"user_id"`
	OperatorID int64 `json:"operator_id"`
}

// ------- SYSTEM -------
type ActivityPayload struct {
	ChatID int64  `json:"chat_id"`
	FromID int64  `json:"from_id"`
	Type   string `json:"type"` // typing, audiomsg, file
}
