package core

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	redis "ovk-im/src/repo/redis"
	"ovk-im/src/repo/search"
	"ovk-im/src/transport/broadcaster"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BaseHandler struct {
	DB          *gorm.DB
	LPRepo      *redis.Repo
	Broadcaster *broadcaster.Broadcaster
	SearchRepo  *search.Repository
}

type VKError struct {
	ErrorCode     int            `json:"error_code"`
	ErrorMsg      string         `json:"error_msg"`
	RequestParams []RequestParam `json:"request_params,omitempty"`
}

type RequestParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *BaseHandler) Reject(c *gin.Context, errorCode int, errorMsg string) {
	params := make([]RequestParam, 0)
	for k, v := range c.Request.URL.Query() {
		params = append(params, RequestParam{Key: k, Value: v[0]})
	}

	c.JSON(http.StatusOK, gin.H{
		"error": VKError{
			ErrorCode:     errorCode,
			ErrorMsg:      errorMsg,
			RequestParams: params,
		},
	})
}

var reAttachment = regexp.MustCompile(`^(photo|video|audio|doc|wall|market|poll|question)-?\d+_\d+$`)

func IsValidAttachments(attachmentStr string) bool {
	if attachmentStr == "" {
		return true
	}

	parts := strings.Split(attachmentStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !reAttachment.MatchString(part) {
			return false
		}
	}
	return true
}

func ParseAttachmentsForLongPoll(attachmentStr string) map[string]interface{} {
	res := make(map[string]interface{})
	if attachmentStr == "" {
		return res
	}

	parts := strings.Split(attachmentStr, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		idx := strconv.Itoa(i + 1)

		typeEnd := 0
		for j, char := range part {
			if (char >= '0' && char <= '9') || char == '-' {
				typeEnd = j
				break
			}
		}

		if typeEnd > 0 {
			res["attach"+idx+"_type"] = part[:typeEnd]
			res["attach"+idx] = part[typeEnd:]
		}
	}

	return res
}

func (r *BaseHandler) BroadcastChatSomethingChanged(ctx *gin.Context, peerID int64, actorID int64) {
	go func(pID, aID int64) {
		ctx := context.Background()
		var memberIDs []int64

		db.Instance.Model(&db_models.ConversationMember{}).
			Where("peer_id = ? AND left_at IS NULL", pID).
			Pluck("user_id", &memberIDs)

		for _, uid := range memberIDs {
			self := 0
			if uid == actorID {
				self = 1
			}

			lpEvent := lp_models.ChatSomethingChangedEvent{
				ChatID: peerID,
				Self:   uint8(self),
			}

			r.LPRepo.PushEvent(ctx, uid, "chat_something_changed", lpEvent)
			r.Broadcaster.Notify(uid)
		}
	}(peerID, actorID)
}

func (r *BaseHandler) BroadcastMarkAsRead(ctx context.Context, peerID int64, userID int64, lastReadID uint64) {
	r.LPRepo.PushEvent(ctx, userID, "read_income_before", lp_models.ReadIncomeBeforeEvent{
		PeerID:  peerID,
		LocalID: lastReadID,
	})
	r.Broadcaster.Notify(userID)

	go func(pID, uID int64, lrID uint64) {
		var members []int64
		db.Instance.Model(&db_models.ConversationMember{}).
			Where("peer_id = ? AND user_id != ? AND left_at IS NULL", peerID, userID).
			Pluck("user_id", &members)

		for _, mID := range members {
			r.LPRepo.PushEvent(ctx, mID, "read_outcome_before", lp_models.ReadOutcomeBeforeEvent{
				PeerID:  userID,
				LocalID: lastReadID,
			})
			r.Broadcaster.Notify(mID)
		}
	}(peerID, userID, lastReadID)
}

func (r *BaseHandler) SendFlagsUpdate(uid int64, chatID string, localID uint64, flags uint64, forAll bool) {
	go func() {
		if forAll {
			var members []int64
			r.DB.Model(&db_models.ConversationMember{}).Where("internal_chat_id = ?", chatID).Pluck("user_id", &members)

			for _, memberID := range members {
				event := lp_models.MsgReplaceFlagsEvent{
					MessageID: localID,
					Flags:     lp_models.MessageFlags{Value: lp_models.MessageFlag(flags)},
					PeerID:    chat.DerivePeerID(chatID, memberID),
				}

				r.LPRepo.PushEvent(context.Background(), memberID, "msg_replace_flags", event)
				r.Broadcaster.Notify(memberID)
			}
		} else {
			event := lp_models.MsgReplaceFlagsEvent{
				MessageID: localID,
				Flags:     lp_models.MessageFlags{Value: lp_models.MessageFlag(flags)},
				PeerID:    chat.DerivePeerID(chatID, uid),
			}

			r.LPRepo.PushEvent(context.Background(), uid, "msg_replace_flags", event)
			r.Broadcaster.Notify(uid)
		}
	}()
}

func (r *BaseHandler) SendUpdateEvent(peerID int64, localID uint64, text string, attachStr string, senderID int64) {
	go func() {
		lpAttach := lp_models.NewLPAttachments(attachStr)
		lpAttach.From = strconv.FormatInt(senderID, 10)
		updateEvent := lp_models.UpdateMessageEvent{
			MessageID:   localID,
			PeerID:      peerID,
			NewText:     text,
			Attachments: &lpAttach,
			Timestamp:   uint64(time.Now().Unix()),
		}

		var recipients []int64
		if peerID > 2000000000 {
			r.DB.Model(&db_models.ConversationMember{}).
				Where("peer_id = ? AND left_at IS NULL", peerID).
				Pluck("user_id", &recipients)
		} else {
			recipients = append(recipients, senderID)
			if peerID != senderID {
				recipients = append(recipients, peerID)
			}
		}

		for _, uid := range recipients {
			r.LPRepo.PushEvent(context.Background(), uid, "msg_update", updateEvent)
			r.Broadcaster.Notify(uid)
		}
	}()
}

func CollectAllEntityIDs(items []db_models.VKApiMessage, userIDs *[]int64, groupIDs *[]int64) {
	uMap := make(map[int64]struct{})
	gMap := make(map[int64]struct{})

	var scan func(m db_models.VKApiMessage)
	scan = func(m db_models.VKApiMessage) {
		if m.FromID > 0 {
			uMap[m.FromID] = struct{}{}
		} else if m.FromID < 0 {
			gMap[-m.FromID] = struct{}{}
		}

		if m.PeerID > 0 && m.PeerID < 2000000000 {
			uMap[m.PeerID] = struct{}{}
		} else if m.PeerID < 0 {
			gMap[-m.PeerID] = struct{}{}
		}

		if m.ReplyMessage != nil {
			scan(*m.ReplyMessage)
		}

		for _, fwd := range m.ForwardMessages {
			scan(fwd)
		}
	}

	for _, itm := range items {
		scan(itm)
	}

	for id := range uMap {
		*userIDs = append(*userIDs, id)
	}
	for id := range gMap {
		*groupIDs = append(*groupIDs, id)
	}
}

func (r *BaseHandler) SendChatFlagsUpdate(userID int64, peerID int64, flags uint64) {
	event := lp_models.ChatReplaceFlagsEvent{
		PeerID: peerID,
		Flags:  flags,
	}
	r.LPRepo.PushEvent(context.Background(), userID, "chat_replace_flags", event)
	r.Broadcaster.Notify(userID)
}

func (r *BaseHandler) UpdateChatFlags(userID int64, peerID int64, mask uint64, mode string) error {
	var member db_models.ConversationMember
	err := r.DB.Where("user_id = ? AND peer_id = ?", userID, peerID).First(&member).Error
	if err != nil {
		return err
	}

	newFlags := uint64(member.Flags)
	switch mode {
	case "set":
		newFlags |= mask
	case "reset":
		newFlags &= ^mask
	case "replace":
		newFlags = mask
	}

	r.DB.Model(&member).Update("flags", newFlags)
	r.SendChatFlagsUpdate(userID, peerID, newFlags)

	return nil
}

func (r *BaseHandler) BackgroundDeleteChat(userID int64, peerID int64, internalChatID string) {
	tx := r.DB.Begin()

	err := tx.Where("chat_id = ?", internalChatID).Delete(&db_models.Message{}).Error
	if err != nil {
		tx.Rollback()
		return
	}

	updates := map[string]interface{}{
		"last_message_id": 0,
		"last_read_id":    0,
	}

	err = tx.Model(&db_models.ConversationMember{}).
		Where("user_id = ? AND peer_id = ?", userID, peerID).
		Updates(updates).Error

	if err != nil {
		tx.Rollback()
		return
	}

	tx.Commit()

	r.SendChatDeleteEvent(userID, peerID)
}

func (r *BaseHandler) SendChatDeleteEvent(userID int64, peerID int64) {
	event := lp_models.MassDeleteMessagesEvent{
		PeerID: peerID,
	}
	r.LPRepo.PushEvent(context.Background(), userID, "chat_delete_all", event)
	r.Broadcaster.Notify(userID)
}
