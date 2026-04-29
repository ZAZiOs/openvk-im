package lp_models

import (
	"encoding/json"
	db_models "ovk-im/src/models/db"
)

type Envelope struct {
	TS      uint64            `json:"ts"`
	Updates []json.RawMessage `json:"updates"`
	PTS     uint64            `json:"pts,omitempty"`
	Failed  int               `json:"failed,omitempty"`
	MinVer  int               `json:"min_version,omitempty"`
	MaxVer  int               `json:"max_version,omitempty"`
}

type LPConfig struct {
	Version int
	Mode    int
}

func (c LPConfig) HasAttachments() bool { return (c.Mode & 2) != 0 }
func (c LPConfig) HasExtended() bool    { return (c.Mode & 8) != 0 }
func (c LPConfig) HasPTS() bool         { return (c.Mode & 32) != 0 }
func (c LPConfig) HasExtra() bool       { return (c.Mode & 64) != 0 }
func (c LPConfig) ReturnRandomID() bool { return (c.Mode & 128) != 0 }

type VKEvent interface {
	ToSlice(cfg LPConfig) []interface{}
}

func Pack(e VKEvent, cfg LPConfig) ([]byte, error) {
	return json.Marshal(e.ToSlice(cfg))
}

type VKGetLongPollHistoryResponse struct {
	History  [][]interface{}        `json:"history"`
	Messages VKApiMessagesWithCount `json:"messages"`
	Profiles []int64                `json:"profiles"`
	Groups   []int64                `json:"groups"`
	NewPTS   uint64                 `json:"new_pts"`
	More     int                    `json:"more,omitempty"`
}

type VKApiMessagesWithCount struct {
	Count int                      `json:"count"`
	Items []db_models.VKApiMessage `json:"items"`
}

// --- Flags ---

type MessageFlag uint64

const (
	FlagUnread       MessageFlag = 1
	FlagOutbox       MessageFlag = 2
	FlagReplied      MessageFlag = 4
	FlagImportant    MessageFlag = 8
	FlagChat         MessageFlag = 16
	FlagFriends      MessageFlag = 32
	FlagSpam         MessageFlag = 64
	FlagDeleteForAll MessageFlag = 64
	FlagDeleted      MessageFlag = 128
	FlagFixed        MessageFlag = 256
	FlagMedia        MessageFlag = 512
	FlagHidden       MessageFlag = 65536
)

type MessageFlags struct {
	Value MessageFlag
}

func (f MessageFlags) Has(flag MessageFlag) bool {
	return (f.Value & flag) != 0
}

func (f *MessageFlags) Add(flag MessageFlag) {
	f.Value |= flag
}

func (f MessageFlags) ToUint64() uint64 {
	return uint64(f.Value)
}

type ChatFlag uint64

const (
	FlagChatImportant  ChatFlag = 1
	FlagChatUnanswered ChatFlag = 2
)

type ChatFlags struct {
	Value ChatFlag
}

func (f ChatFlags) Has(flag ChatFlag) bool {
	return (f.Value & flag) != 0
}

func (f *ChatFlags) Add(flag ChatFlag) {
	f.Value |= flag
}

func (f ChatFlags) ToUint64() uint64 {
	return uint64(f.Value)
}

// --- Events ---
/*
  web archive marks:

  v0 https://web.archive.org/web/20131031232329/http://vk.com/dev/using_longpoll

  ^^^ v0 Требует больших костыльных доработок чтобы работало. Поэтому совместимость не гарантирована.

  v1 https://web.archive.org/web/20170205084502/https://vk.com/dev/using_longpoll

  v2 https://web.archive.org/web/20190405024658/https://vk.com/dev/using_longpoll

  v3 - latest
*/

// v0
type MsgDeleteEvent struct {
	MessageID uint64
}

func (e MsgDeleteEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{0, e.MessageID, 0}
}

// v0
type MsgReplaceFlagsEvent struct {
	MessageID uint64
	Flags     MessageFlags
	PeerID    int64
}

func (e MsgReplaceFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{1, e.MessageID, e.Flags.ToUint64(), e.PeerID}
}

// v0
type MsgSetFlagsEvent struct {
	MessageID uint64
	Mask      MessageFlags
	PeerID    int64
}

func (e MsgSetFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{2, e.MessageID, e.Mask.ToUint64(), e.PeerID}
}

// v0
type MsgResetFlagsEvent struct {
	MessageID uint64
	Mask      MessageFlags
	PeerID    int64
}

func (e MsgResetFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{3, e.MessageID, e.Mask.ToUint64(), e.PeerID}
}

// v0
type NewMessageEvent struct {
	MessageID   uint64
	Flags       MessageFlags
	MinorID     int64
	PeerID      int64
	Timestamp   int
	Text        string
	Attachments map[string]interface{}
	RandomID    int
}

func (e NewMessageEvent) ToSlice(cfg LPConfig) []interface{} {
	var peerID int64
	if cfg.Version == 0 && e.PeerID < 0 {
		// Старый формат для сообществ в v0: 1000000000 + group_id
		// e.PeerID у нас отрицательный для сообществ
		peerID = 1000000000 + (-e.PeerID)
	} else {
		// В v1+ или для юзеров/чатов используем как есть
		peerID = e.PeerID
	}

	res := []interface{}{
		4,                  // code
		e.MessageID,        // $message_id
		e.Flags.Value,      // $flags
		peerID,             // $from_id / $peer_id
		e.Timestamp,        // $timestamp
		" [NotSupported] ", // $subject (Имя беседы / Email, необязательно)
		e.Text,             // $text
	}

	// mode=2:
	if cfg.HasAttachments() {
		if e.Attachments == nil {
			res = append(res, map[string]interface{}{})
		} else {
			res = append(res, e.Attachments)
		}
	}

	if cfg.ReturnRandomID() {
		if e.RandomID != 0 {
			res = append(res, e.RandomID)
		}
	}

	return res
}

// v2
type UpdateMessageEvent struct {
	MessageId   uint64
	Mask        MessageFlags // Работает как set.
	PeerId      int64
	Timestamp   uint64
	NewText     string
	Attachments map[string]interface{}
	Stub        uint8
}

func (e UpdateMessageEvent) ToSlice(cfg LPConfig) []interface{} {
	attachments := e.Attachments
	if attachments == nil {
		attachments = map[string]interface{}{}
	}
	return []interface{}{5, e.MessageId, e.Mask.ToUint64(), e.PeerId, e.Timestamp, e.NewText, attachments, e.Stub}
}

// v1
type ReadIncomeBeforeEvent struct {
	PeerID  int64
	LocalID uint64
}

func (e ReadIncomeBeforeEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{6, e.PeerID, e.LocalID}
}

// v1
type ReadOutcomeBeforeEvent struct {
	PeerID  int64
	LocalID uint64
}

func (e ReadOutcomeBeforeEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{7, e.PeerID, e.LocalID}
}

// v0
type GotOnlineEvent struct {
	UserID    int64
	Extra     int64  // != 0; last byte =
	Timestamp uint64 // Last action at
}

func (e GotOnlineEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{8, e.UserID, e.Extra, e.Timestamp}
}

// v0
type GotOfflineEvent struct {
	UserID    int64
	Flags     int64  // 0 - Left site; 1 - Timeout
	Timestamp uint64 // Last action at
}

func (e GotOfflineEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{9, e.UserID, e.Flags, e.Timestamp}
}

// v1
type ChatResetFlagsEvent struct {
	PeerID int64
	Mask   uint64
}

func (e ChatResetFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{10, e.PeerID, e.Mask}
}

// v1
type ChatReplaceFlagsEvent struct {
	PeerID int64
	Flags  uint64
}

func (e ChatReplaceFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{11, e.PeerID, e.Flags}
}

// v1
type ChatSetFlagsEvent struct {
	PeerID int64
	Mask   uint64
}

func (e ChatSetFlagsEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{12, e.PeerID, e.Mask}
}

// v2
type MassDeleteMessagesEvent struct {
	PeerID  int64
	LocalID uint64
}

func (e MassDeleteMessagesEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{13, e.PeerID, e.LocalID}
}

// v3
type MassRestoreMessagesEvent struct {
	PeerID  int64
	LocalID uint64
}

func (e MassRestoreMessagesEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{14, e.PeerID, e.LocalID}
}

// Indicates a global change in dialogue history. Local cache should be re-synced via messages.getHistory.
// v3
type StateSyncEvent struct {
	PeerID  int64
	MajorID uint64
}

func (e StateSyncEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{20, e.PeerID, e.MajorID}
}

// Indicates minor dialogue metadata changes. Does not usually require full history re-sync.
// v3
type MetaDataSyncEvent struct {
	PeerID  int64
	MinorID uint64
}

func (e MetaDataSyncEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{21, e.PeerID, e.MinorID}
}

// v0
type ChatSomethingChangedEvent struct {
	ChatID int64
	Self   uint8 // Is triggered by same user 1 or 0
}

func (e ChatSomethingChangedEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{51, e.ChatID, e.Self}
}

// v3
type ChatUpdateEvent struct {
	TypeID int64
	PeerID int64
}

func (e ChatUpdateEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{52, e.TypeID, e.PeerID}
}

// v0
type IsDMTypingEvent struct {
	UserID int64
	Flags  uint8 // 0 - Stopped, 1 - Typing
}

func (e IsDMTypingEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{61, e.UserID, e.Flags}
}

// v0
type IsChatTypingEvent struct {
	UserID int64
	ChatID int64
}

func (e IsChatTypingEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{62, e.UserID, e.ChatID}
}

// v3
type MultiUsersTypingEvent struct {
	UserIDs    []int64
	PeerID     int64
	TotalCount int
	Timestamp  int64
}

func (e MultiUsersTypingEvent) ToSlice(cfg LPConfig) []interface{} {
	userIDs := e.UserIDs
	if userIDs == nil {
		userIDs = []int64{}
	}
	return []interface{}{63, userIDs, e.PeerID, e.TotalCount, e.Timestamp}
}

// v3
type MultiUsersAudioRecordingEvent struct {
	UserIDs    []int64
	PeerID     int64
	TotalCount int
	Timestamp  int64
}

func (e MultiUsersAudioRecordingEvent) ToSlice(cfg LPConfig) []interface{} {
	userIDs := e.UserIDs
	if userIDs == nil {
		userIDs = []int64{}
	}
	return []interface{}{64, userIDs, e.PeerID, e.TotalCount, e.Timestamp}
}

// ЗВОНКИ В ОПЕНВК?!
// v0
type MakingACallEvent struct {
	UserID int64
	CallID uint64
}

func (e MakingACallEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{70, e.UserID, e.CallID}
}

// v1
type CounterUpdateEvent struct {
	Count uint
}

func (e CounterUpdateEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{80, e.Count}
}

// v1
type NotificationSetEvent struct {
	PeerID        int64
	Sound         uint8 // 0 - Notif off; 1 - Notif on;
	DisabledUntil int64 // Timestamp until; -1 - forever
}

func (e NotificationSetEvent) ToSlice(cfg LPConfig) []interface{} {
	return []interface{}{114, e.PeerID, e.Sound, e.DisabledUntil}
}
