package chats

import (
	"context"
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func EditChat(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	title := strings.TrimSpace(c.Query("title"))

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}
	if title == "" {
		r.Reject(c, 100, "One of the parameters is missing: title")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	if !strings.HasPrefix(internalChatID, "c") {
		r.Reject(c, 100, "Bad request: current peer_id is not a group chat")
		return
	}

	var check db_models.ConversationMember
	err := db.Instance.Where(
		"internal_chat_id = ? AND user_id = ? AND is_admin = ? AND left_at IS NULL",
		internalChatID,
		currentUserID,
		true,
	).First(&check).Error

	if err != nil {
		r.Reject(c, 917, "You don't have access to this chat or you are not an administrator of this chat")
		return
	}

	err = db.Instance.Model(&db_models.Conversation{}).
		Where("internal_id = ?", internalChatID).
		Update("title", title).Error

	if err != nil {
		r.Reject(c, 10, "Failed to update chat title: "+err.Error())
		return
	}

	messageText := "changed conversation title to «" + title + "»"
	msg, err := chat.CreateServiceMessage(
		currentUserID,
		internalChatID,
		messageText,
		"chat_title_update",
		0,
		title,
	)
	if err != nil {
		r.Reject(c, 10, "Failed to create service message: "+err.Error())
		return
	}

	var members []db_models.ConversationMember
	db.Instance.Where("internal_chat_id = ? AND left_at IS NULL", internalChatID).Find(&members)

	var participants []int64
	for _, m := range members {
		participants = append(participants, m.UserID)
	}

	hasEmoji := false
	for _, runeVal := range messageText {
		if runeVal > 0x2000 {
			hasEmoji = true
			break
		}
	}
	lpAttach := lp_models.LPAttachments{
		Source: "chat_title_update",
		Mid:    strconv.FormatInt(currentUserID, 10),
		Emoji:  hasEmoji,
		From:   strconv.FormatInt(currentUserID, 10),
	}
	baseMsgEvent := lp_models.NewMessageEvent{
		MessageID:   uint64(msg.ID),
		PeerID:      peerID,
		Timestamp:   int(msg.CreatedAt.Unix()),
		Text:        messageText,
		Attachments: &lpAttach,
	}

	go func(participants []int64, msgEvent lp_models.NewMessageEvent) {
		ctx := context.Background()

		for _, uID := range participants {
			var selfFlag uint8 = 0
			if uID == currentUserID {
				selfFlag = 1
			}

			// --- EVENT 1: Chat something changed (51) ---
			updateEvent := lp_models.ChatSomethingChangedEvent{
				ChatID: peerID,
				Self:   selfFlag,
			}
			r.LPRepo.PushEvent(ctx, uID, "chat_update", updateEvent)

			// --- EVENT 2: New service message (4) ---
			userMsgEvent := msgEvent
			userMsgEvent.Flags = lp_models.MessageFlags{Value: msgEvent.Flags.Value}
			if uID == currentUserID {
				userMsgEvent.Flags.Add(lp_models.FlagOutbox)
			} else {
				userMsgEvent.Flags.Add(lp_models.FlagUnread)
			}
			r.LPRepo.PushEvent(ctx, uID, "new_msg", userMsgEvent)

			r.Broadcaster.Notify(uID)
		}
	}(participants, baseMsgEvent)

	c.JSON(http.StatusOK, gin.H{
		"response": 1,
	})
}
