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
	/*  Я не нашёл подтверждения какой именно chatID юзался. Поэтому я пропихну peerID

	В v0-v2 чаты в LongPoll часто передавались как chat_id (peer_id - 2000000000)
	chatID := peerID
	if peerID > 2000000000 {
		chatID = peerID - 2000000000
	}
	*/

	var memberIDs []int64
	db.Instance.Model(&db_models.ConversationMember{}).
		Where("peer_id = ? AND left_at IS NULL", peerID).
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
}

func (r *BaseHandler) BroadcastMarkAsRead(ctx *gin.Context, peerID int64, userID int64, lastReadID uint64) {
	r.LPRepo.PushEvent(ctx, userID, "read_income_before", lp_models.ReadIncomeBeforeEvent{
		PeerID:  peerID,
		LocalID: lastReadID,
	})
	r.Broadcaster.Notify(userID)

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
}

func (r *BaseHandler) SendFlagsUpdate(uid int64, peerID int64, localID uint64, flags uint64, forAll bool) {
	event := lp_models.MsgReplaceFlagsEvent{
		MessageID: localID,
		Flags:     lp_models.MessageFlags{Value: lp_models.MessageFlag(flags)},
		PeerID:    peerID,
	}

	if forAll {
		var members []int64
		r.DB.Model(&db_models.ConversationMember{}).Where("peer_id = ?", peerID).Pluck("user_id", &members)

		for _, memberID := range members {
			r.LPRepo.PushEvent(context.Background(), memberID, "msg_flags_replace", event)
			r.Broadcaster.Notify(memberID)
		}
	} else {
		r.LPRepo.PushEvent(context.Background(), uid, "msg_flags_replace", event)
		r.Broadcaster.Notify(uid)
	}
}

func (r *BaseHandler) SendUpdateEvent(peerID int64, localID uint64, text string, attachStr string, senderID int64) {
	lpAttach := lp_models.NewLPAttachments(attachStr)

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
