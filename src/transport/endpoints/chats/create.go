package chats

import (
	"context"
	"net/http"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func CreateChat(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	title := c.Query("title")
	userIDsRaw := c.Query("user_ids")

	if title == "" {
		r.Reject(c, 100, "One of the parameters is missing: title")
		return
	}
	if userIDsRaw == "" {
		r.Reject(c, 100, "One of the parameters is missing: user_ids")
		return
	}

	rawIDs := strings.Split(userIDsRaw, ",")
	var userIDs []int64
	for _, idStr := range rawIDs {
		idStr = strings.TrimSpace(idStr)
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			if id != currentUserID {
				userIDs = append(userIDs, id)
			}
		}
	}

	if len(userIDs) == 0 {
		r.Reject(c, 100, "At least one other user_id is required")
		return
	}

	conv, err := chat.CreateConversation(currentUserID, userIDs, title)
	if err != nil {
		r.Reject(c, 10, "Internal server error: "+err.Error())
		return
	}

	messageText := "created conversation «" + title + "»"
	msg, err := chat.CreateServiceMessage(
		currentUserID,
		conv.InternalID,
		messageText,
		"chat_create",
		0,
		title,
	)

	if err != nil {
		r.Reject(c, 10, "Failed to create service message: "+err.Error())
		return
	}

	hasEmoji := false
	for _, runeVal := range messageText {
		if runeVal > 0x2000 {
			hasEmoji = true
			break
		}
	}

	lpAttach := lp_models.LPAttachments{
		Source: "chat_create",
		Mid:    strconv.FormatInt(currentUserID, 10),
		Emoji:  hasEmoji,
		From:   strconv.FormatInt(currentUserID, 10),
	}

	baseEvent := lp_models.NewMessageEvent{
		MessageID:   uint64(msg.ID),
		PeerID:      conv.PeerID,
		Timestamp:   int(msg.CreatedAt.Unix()),
		Text:        messageText,
		Attachments: &lpAttach,
	}

	allParticipants := append(userIDs, currentUserID)

	go func(participants []int64, event lp_models.NewMessageEvent) {
		ctx := context.Background()

		for _, uID := range participants {
			userEvent := event
			userEvent.Flags = lp_models.MessageFlags{Value: event.Flags.Value}

			if uID == currentUserID {
				userEvent.Flags.Add(lp_models.FlagOutbox)
			} else {
				userEvent.Flags.Add(lp_models.FlagUnread)
			}

			_, _, err := r.LPRepo.PushEvent(ctx, uID, "new_msg", userEvent)
			if err == nil {
				r.Broadcaster.Notify(uID)
			}
		}
	}(allParticipants, baseEvent)

	c.JSON(http.StatusOK, gin.H{
		"response": conv.PeerID - 2000000000,
	})
}
