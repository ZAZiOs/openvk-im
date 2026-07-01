package chats

import (
	"context"
	"net/http"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func AddChatUser(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)

	if peerID == 0 || userID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or user_id")
		return
	}

	if peerID < 2000000000 {
		r.Reject(c, 15, "Access denied: cannot add user to direct message")
		return
	}
	chatID := chat.GetInternalChatID(peerID, currentUserID)

	err := chat.AddUserToConversation(chatID, userID, currentUserID)
	if err != nil {
		r.Reject(c, 10, "Internal server error: failed to add user: "+err.Error())
		return
	}

	messageText := "invited user " + strconv.FormatInt(userID, 10)
	msg, err := chat.CreateServiceMessage(
		currentUserID,
		chatID,
		messageText,
		"chat_invite_user",
		userID,
		"",
	)

	if err == nil {
		participants, err := chat.GetActiveMemberIDs(nil, chatID)
		if err == nil {
			hasEmoji := false
			lpAttach := lp_models.LPAttachments{
				Source: "chat_invite_user",
				From:   strconv.FormatInt(currentUserID, 10),
				Emoji:  hasEmoji,
			}

			baseEvent := lp_models.NewMessageEvent{
				MessageID:   uint64(msg.ID),
				PeerID:      peerID,
				Timestamp:   int(msg.CreatedAt.Unix()),
				Text:        messageText,
				Attachments: &lpAttach,
			}

			go func(parts []int64, event lp_models.NewMessageEvent) {
				ctx := context.Background()
				for _, uID := range parts {
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
			}(participants, baseEvent)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response": 1,
	})
}

func RemoveChatUser(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	if userID == 0 {
		userID = currentUserID
	}

	if peerID < 2000000000 {
		r.Reject(c, 15, "Access denied: cannot remove user from direct message")
		return
	}
	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if userID != currentUserID {
		member, err := chat.GetMember(nil, chatID, currentUserID)
		if err != nil || member == nil {
			r.Reject(c, 15, "Access denied: you are not in this chat")
			return
		}

		if !member.IsAdmin {
			r.Reject(c, 15, "Access denied: you must be an admin to kick other users")
			return
		}
	}

	isActive, err := chat.IsUserInChat(nil, chatID, userID)
	if err != nil || !isActive {
		r.Reject(c, 15, "User is not in the chat")
		return
	}

	err = chat.RemoveUserFromConversation(nil, chatID, userID)
	if err != nil {
		r.Reject(c, 10, "Internal server error: failed to remove user: "+err.Error())
		return
	}

	messageText := "kicked user " + strconv.FormatInt(userID, 10)
	if userID == currentUserID {
		messageText = "left the chat"
	}

	msg, err := chat.CreateServiceMessage(
		currentUserID,
		chatID,
		messageText,
		"chat_kick_user",
		userID,
		"",
	)

	if err == nil {
		participants, err := chat.GetActiveMemberIDs(nil, chatID)
		if err == nil {
			notifyList := append(participants, userID)

			hasEmoji := false
			lpAttach := lp_models.LPAttachments{
				Source: "chat_kick_user",
				From:   strconv.FormatInt(currentUserID, 10),
				Emoji:  hasEmoji,
			}

			baseEvent := lp_models.NewMessageEvent{
				MessageID:   uint64(msg.ID),
				PeerID:      peerID,
				Timestamp:   int(msg.CreatedAt.Unix()),
				Text:        messageText,
				Attachments: &lpAttach,
			}

			go func(parts []int64, event lp_models.NewMessageEvent) {
				ctx := context.Background()
				for _, uID := range parts {
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
			}(notifyList, baseEvent)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response": 1,
	})
}
