package messages

import (
	"context"
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func Pin(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	messageID, _ := strconv.ParseUint(c.Query("message_id"), 10, 64)
	cmid, _ := strconv.ParseUint(c.Query("conversation_message_id"), 10, 64)
	if messageID == 0 && cmid != 0 {
		messageID = cmid
	}

	if peerID == 0 || messageID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or message_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, chatID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	} else {
		inChat, _ := chat.IsUserInChat(db.Instance, chatID, currentUserID)
		if !inChat {
			r.Reject(c, 917, "Access denied")
			return
		}
	}

	var msg db_models.Message
	q := db.Instance.Where("chat_id = ? AND (local_id = ? OR id = ?)", chatID, messageID, messageID)
	q = db_models.BuildVisibilityFilter(q, chatID, currentUserID)
	if err := q.First(&msg).Error; err != nil {
		r.Reject(c, 946, "Message not found")
		return
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("internal_id = ?", chatID).
		Update("pinned_msg_id", msg.LocalID).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	messageText := "pinned a message"
	serviceMsg, err := chat.CreateServiceMessage(
		currentUserID,
		chatID,
		messageText,
		"chat_pin_message",
		msg.FromID,
		strconv.FormatUint(msg.LocalID, 10),
	)
	if err == nil && serviceMsg != nil {
		members, _ := chat.GetActiveMemberIDs(nil, chatID)
		lpAttach := lp_models.LPAttachments{
			Source: "chat_pin_message",
			From:   strconv.FormatInt(currentUserID, 10),
			Mid:    strconv.FormatInt(msg.FromID, 10),
		}

		baseEvent := lp_models.NewMessageEvent{
			MessageID:   uint64(serviceMsg.ID),
			MinorID:     int64(serviceMsg.LocalID),
			PeerID:      peerID,
			Timestamp:   int(serviceMsg.CreatedAt.Unix()),
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
		}(members, baseEvent)
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"response": msg.ToVKApiStruct(db.Instance, 0, currentUserID, peerID),
	})
}

func Unpin(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID > 2000000000 {
		member, err := chat.GetMember(db.Instance, chatID, currentUserID)
		if err != nil || member == nil || !member.IsAdmin {
			r.Reject(c, 925, "You are not admin of this chat")
			return
		}
	}

	err := db.Instance.Model(&db_models.Conversation{}).
		Where("internal_id = ?", chatID).
		Update("pinned_msg_id", 0).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	messageText := "unpinned message"
	serviceMsg, err := chat.CreateServiceMessage(
		currentUserID,
		chatID,
		messageText,
		"chat_unpin_message",
		currentUserID,
		"",
	)
	if err == nil && serviceMsg != nil {
		members, _ := chat.GetActiveMemberIDs(nil, chatID)
		lpAttach := lp_models.LPAttachments{
			Source: "chat_unpin_message",
			From:   strconv.FormatInt(currentUserID, 10),
		}

		baseEvent := lp_models.NewMessageEvent{
			MessageID:   uint64(serviceMsg.ID),
			MinorID:     int64(serviceMsg.LocalID),
			PeerID:      peerID,
			Timestamp:   int(serviceMsg.CreatedAt.Unix()),
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
		}(members, baseEvent)
	}

	r.BroadcastChatSomethingChanged(c, peerID, currentUserID)

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func GetPinnedMessage(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerIDStr := c.Query("peer_id")
	uIDParam, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	peerID, _ := strconv.ParseInt(peerIDStr, 10, 64)

	chatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)
	if chatID == "" && peerID != 0 {
		chatID = chat.GetInternalChatID(peerID, currentUserID)
	}

	if chatID == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	if peerID == 0 {
		peerID = chat.DerivePeerID(chatID, currentUserID)
	}

	if currentUserID != 0 {
		inChat, _ := chat.IsUserInChat(db.Instance, chatID, currentUserID)
		if !inChat {
			r.Reject(c, 917, "Access denied")
			return
		}
	}

	var conv db_models.Conversation
	if err := db.Instance.Where("internal_id = ?", chatID).First(&conv).Error; err != nil || conv.PinnedMsgID == 0 {
		res := gin.H{
			"count": 0,
			"items": []interface{}{},
		}
		if c.Query("extended") == "1" {
			res["profiles"] = []int64{}
			res["groups"] = []int64{}
		}
		c.JSON(http.StatusOK, gin.H{"response": res})
		return
	}

	var msg db_models.Message
	q := db.Instance.Where("chat_id = ? AND local_id = ?", chatID, conv.PinnedMsgID)
	q = db_models.BuildVisibilityFilter(q, chatID, currentUserID)
	if err := q.First(&msg).Error; err != nil {
		res := gin.H{
			"count": 0,
			"items": []interface{}{},
		}
		if c.Query("extended") == "1" {
			res["profiles"] = []int64{}
			res["groups"] = []int64{}
		}
		c.JSON(http.StatusOK, gin.H{"response": res})
		return
	}

	vkMsg := msg.ToVKApiStruct(db.Instance, 5, currentUserID, peerID)

	response := gin.H{
		"count":          1,
		"items":          []db_models.VKApiMessage{vkMsg},
		"pinned_message": vkMsg,
	}

	if c.Query("extended") == "1" {
		var userIDs []int64
		var groupIDs []int64

		core.CollectAllEntityIDs([]db_models.VKApiMessage{vkMsg}, &userIDs, &groupIDs)

		response["profiles"] = userIDs
		response["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
