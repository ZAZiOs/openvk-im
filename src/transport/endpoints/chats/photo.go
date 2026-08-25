package chats

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"ovk-im/src/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func SetChatPhoto(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if cID := c.Query("chat_id"); cID != "" {
		id, _ := strconv.ParseInt(cID, 10, 64)
		peerID = 2000000000 + id
	}

	if peerID <= 2000000000 {
		r.Reject(c, 100, "One of the parameters is missing or invalid: peer_id / chat_id")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	if !strings.HasPrefix(internalChatID, "c") {
		r.Reject(c, 100, "Bad request: target is not a group chat")
		return
	}

	member, err := chat.GetMember(db.Instance, internalChatID, currentUserID)
	if err != nil || member == nil || member.LeftAt != nil {
		r.Reject(c, 917, "You don't have access to this chat")
		return
	}

	messageText := "updated chat photo"
	msg, err := chat.CreateServiceMessage(
		currentUserID,
		internalChatID,
		messageText,
		"chat_photo_update",
		currentUserID,
		"",
	)
	if err != nil {
		r.Reject(c, 10, "Failed to create service message: "+err.Error())
		return
	}

	participants, err := chat.GetActiveMemberIDs(nil, internalChatID)
	if err == nil {
		lpAttach := lp_models.LPAttachments{
			Source: "chat_photo_update",
			From:   strconv.FormatInt(currentUserID, 10),
			Mid:    strconv.FormatInt(currentUserID, 10),
		}

		baseEvent := lp_models.NewMessageEvent{
			MessageID:   msg.LocalID,
			PeerID:      peerID,
			Timestamp:   int(msg.CreatedAt.Unix()),
			Text:        messageText,
			Attachments: &lpAttach,
		}

		go func(parts []int64, event lp_models.NewMessageEvent) {
			ctx := context.Background()
			for _, uID := range parts {
				var selfFlag uint8 = 0
				if uID == currentUserID {
					selfFlag = 1
				}

				// LP Event 51: chat updated
				r.LPRepo.PushEvent(ctx, uID, "chat_something_changed", lp_models.ChatSomethingChangedEvent{
					ChatID: peerID,
					Self:   selfFlag,
				})

				// LP Event 4: new service message
				userEvent := event
				userEvent.Flags = lp_models.MessageFlags{Value: event.Flags.Value}
				if uID == currentUserID {
					userEvent.Flags.Add(lp_models.FlagOutbox)
				} else {
					userEvent.Flags.Add(lp_models.FlagUnread)
				}

				r.LPRepo.PushEvent(ctx, uID, "new_msg", userEvent)
				r.Broadcaster.Notify(uID)
			}
		}(participants, baseEvent)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"message_id": msg.LocalID,
		},
	})
}

func DeleteChatPhoto(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if cID := c.Query("chat_id"); cID != "" {
		id, _ := strconv.ParseInt(cID, 10, 64)
		peerID = 2000000000 + id
	}

	if peerID <= 2000000000 {
		r.Reject(c, 100, "One of the parameters is missing or invalid: peer_id / chat_id")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	if !strings.HasPrefix(internalChatID, "c") {
		r.Reject(c, 100, "Bad request: target is not a group chat")
		return
	}

	member, err := chat.GetMember(db.Instance, internalChatID, currentUserID)
	if err != nil || member == nil || member.LeftAt != nil {
		r.Reject(c, 917, "You don't have access to this chat")
		return
	}

	messageText := "removed chat photo"
	msg, err := chat.CreateServiceMessage(
		currentUserID,
		internalChatID,
		messageText,
		"chat_photo_remove",
		currentUserID,
		"",
	)
	if err != nil {
		r.Reject(c, 10, "Failed to create service message: "+err.Error())
		return
	}

	participants, err := chat.GetActiveMemberIDs(nil, internalChatID)
	if err == nil {
		lpAttach := lp_models.LPAttachments{
			Source: "chat_photo_remove",
			From:   strconv.FormatInt(currentUserID, 10),
			Mid:    strconv.FormatInt(currentUserID, 10),
		}

		baseEvent := lp_models.NewMessageEvent{
			MessageID:   msg.LocalID,
			PeerID:      peerID,
			Timestamp:   int(msg.CreatedAt.Unix()),
			Text:        messageText,
			Attachments: &lpAttach,
		}

		go func(parts []int64, event lp_models.NewMessageEvent) {
			ctx := context.Background()
			for _, uID := range parts {
				var selfFlag uint8 = 0
				if uID == currentUserID {
					selfFlag = 1
				}

				r.LPRepo.PushEvent(ctx, uID, "chat_something_changed", lp_models.ChatSomethingChangedEvent{
					ChatID: peerID,
					Self:   selfFlag,
				})

				userEvent := event
				userEvent.Flags = lp_models.MessageFlags{Value: event.Flags.Value}
				if uID == currentUserID {
					userEvent.Flags.Add(lp_models.FlagOutbox)
				} else {
					userEvent.Flags.Add(lp_models.FlagUnread)
				}

				r.LPRepo.PushEvent(ctx, uID, "new_msg", userEvent)
				r.Broadcaster.Notify(uID)
			}
		}(participants, baseEvent)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"message_id": msg.LocalID,
		},
	})
}
