package messages

import (
	"context"
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"strconv"
	"strings"
	"time"

	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MessagesHandler struct {
	core.BaseHandler
}

func Send(c *gin.Context, r *core.BaseHandler) {
	var peerID int64
	val, exists := c.Get("userID")
	if !exists || val == nil {
		return
	}
	currentUserID := val.(int64)

	if uID := c.Query("user_id"); uID != "" {
		id, _ := strconv.ParseInt(uID, 10, 64)
		peerID = id
	} else if cID := c.Query("chat_id"); cID != "" {
		id, _ := strconv.ParseInt(cID, 10, 64)
		peerID = 2000000000 + id
	} else if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: user_id, chat_id or peer_id")
		return
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	isGroupChat := strings.HasPrefix(internalChatID, "c")

	// ------------------------------------

	message := c.Query("message")
	attachment := c.Query("attachment")
	randomIDStr := c.Query("random_id")
	replyToStr := c.Query("reply_to")
	forwardMessagesRaw := c.Query("forward_messages")

	// Message, Attachment, Reply or Forward
	if message == "" && attachment == "" && replyToStr == "" && forwardMessagesRaw == "" {
		r.Reject(c, 100, "One of the parameters is missing: message, attachment, reply_to or forward_messages")
		return
	}

	// Message
	if len(message) > 9000 {
		r.Reject(c, 914, "Message is too long")
		return
	}

	// Attachment
	if attachment != "" {
		if !core.IsValidAttachments(attachment) {
			r.Reject(c, 100, "Invalid attachment format")
			return
		}
	}

	// RandomID
	if randomIDStr == "" {
		r.Reject(c, 100, "random_id is required")
		return
	}
	randomID, _ := strconv.ParseInt(randomIDStr, 10, 64)
	redisKey := "rid:" + strconv.FormatInt(currentUserID, 10) + ":" + strconv.FormatInt(peerID, 10) + randomIDStr

	oldLocalID, err := r.LPRepo.Client.Get(c.Request.Context(), redisKey).Result()
	if err == nil && oldLocalID != "" {
		lID, _ := strconv.ParseUint(oldLocalID, 10, 64)
		c.JSON(http.StatusOK, gin.H{
			"response": lID,
		})
		return
	}

	// ReplyTo
	var replyTo uint64
	if replyToStr != "" {
		id, _ := strconv.ParseUint(replyToStr, 10, 64)
		replyTo = id
	}

	// Forward
	if forwardMessagesRaw != "" {
		ids := strings.Split(forwardMessagesRaw, ",")
		if len(ids) > 100 {
			r.Reject(c, 100, "Too many forward_messages")
			return
		}
	}

	// ------------------------------------
	// ПРОВЕРКА ПРАВ: Кто кому может писать
	// ------------------------------------

	if currentUserID < 0 && peerID < 0 {
		r.Reject(c, 100, "Communities cannot send messages to other communities")
		return
	}

	switch {
	case isGroupChat:
		// ПОЛУЧАТЕЛЬ: ЧАТ
		inChat, err := chat.IsUserInChat(nil, internalChatID, currentUserID)
		if err != nil || !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}

	case peerID < 0:
		// ПОЛУЧАТЕЛЬ: СООБЩЕСТВО
		// Тут обычно проверяется, не забанило ли сообщество юзера,
		// но как минимум, мы разрешаем юзеру сюда писать.
		if currentUserID < 0 { // Дублирующая проверка на всякий случай
			r.Reject(c, 100, "Communities cannot send messages to other communities")
			return
		}

	case peerID > 0:
		// ПОЛУЧАТЕЛЬ: ПОЛЬЗОВАТЕЛЬ (личка)

		// Если пишет СООБЩЕСТВО пользователю
		if currentUserID < 0 {
			var exists bool
			dbx.Instance.Model(&db_models.Message{}).
				Select("count(*) > 0").
				Where("chat_id = ? AND from_id = ?", internalChatID, peerID).
				Scan(&exists)

			if !exists {
				r.Reject(c, 901, "Can't send messages for users without permission")
				return
			}
		}

	default:
		r.Reject(c, 100, "Invalid peer_id")
		return
	}

	// ------------------------------------------------

	var finalLocalID uint64
	err = dbx.Instance.Transaction(func(tx *gorm.DB) error {
		localID, err := chat.NextLocalID(tx, internalChatID, currentUserID)
		if err != nil {
			return err
		}
		finalLocalID = localID

		if !isGroupChat {
			tx.FirstOrCreate(&db_models.ConversationMember{
				InternalChatID: internalChatID,
				UserID:         currentUserID,
				JoinedAt:       time.Now(),
				IsAdmin:        true,
			})
			chat.EnsureMemberPeriod(tx, internalChatID, currentUserID, 1)

			if peerID != currentUserID {
				tx.FirstOrCreate(&db_models.ConversationMember{
					InternalChatID: internalChatID,
					UserID:         peerID,
					JoinedAt:       time.Now(),
					IsAdmin:        true,
				})
				chat.EnsureMemberPeriod(tx, internalChatID, peerID, 1)
			}
		}

		newMessage := db_models.Message{
			ChatID:          internalChatID,
			LocalID:         localID,
			FromID:          currentUserID,
			Text:            db_models.EncryptedJSON(message),
			Attachments:     db_models.EncryptedJSON(attachment),
			ReplyTo:         &replyTo,
			RandomID:        randomID,
			ForwardMessages: forwardMessagesRaw,
			CreatedAt:       time.Now(),
		}

		if err := tx.Create(&newMessage).Error; err != nil {
			return err
		}

		if err := tx.Model(&db_models.Conversation{}).
			Where("internal_id = ?", internalChatID).
			Update("last_message_id", localID).Error; err != nil {
			return err
		}

		if err := tx.Model(&db_models.ConversationMember{}).
			Where("internal_chat_id = ?", internalChatID).
			Update("last_message_id", localID).Error; err != nil {
			return err
		}

		tx.Model(&db_models.ConversationMember{}).
			Where("internal_chat_id = ? AND user_id = ?", internalChatID, currentUserID).
			Update("last_read_id", localID)

		if message != "" {
			indexes := r.SearchRepo.GenerateBlindIndexes(newMessage.ID, internalChatID, message)
			if len(indexes) > 0 {
				if err := tx.Create(&indexes).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		switch err {
		case chat.ErrGroupNoPermission:
			r.Reject(c, 901, "Can't send messages for users without permission")
		case chat.ErrChatNotFound:
			r.Reject(c, 917, "Chat not found or access denied")
		default:
			r.Reject(c, 10, "Internal server error: "+err.Error())
		}
		return
	}

	r.LPRepo.Client.Set(c.Request.Context(), redisKey, finalLocalID, 24*time.Hour)

	// --- ПОДГОТОВКА LONGPOLL СОБЫТИЯ ---
	lpAttach := lp_models.NewLPAttachments(attachment)
	lpAttach.From = strconv.FormatInt(currentUserID, 10)
	if replyTo != 0 {
		lpAttach.ReplyTo = strconv.FormatUint(replyTo, 10)
	}
	if forwardMessagesRaw != "" {
		lpAttach.Fwd = forwardMessagesRaw
	}
	// TODO: Добавить проверку на emoji

	lpEvent := lp_models.NewMessageEvent{
		MessageID:   finalLocalID,
		Flags:       lp_models.MessageFlags{Value: 0},
		PeerID:      peerID,
		Timestamp:   int(time.Now().Unix()),
		Text:        message,
		Attachments: &lpAttach,
		RandomID:    int(randomID),
	}

	var recipients []int64
	if isGroupChat {
		dbx.Instance.Model(&db_models.ConversationMember{}).
			Where("internal_chat_id = ? AND left_at IS NULL", internalChatID).
			Pluck("user_id", &recipients)
	} else {
		recipients = append(recipients, currentUserID)
		if peerID != currentUserID {
			recipients = append(recipients, peerID)
		}
	}

	go func(rcps []int64, event lp_models.NewMessageEvent, currentID int64) {
		ctx := context.Background()

		for _, uid := range rcps {
			userEvent := event
			userEvent.Flags = lp_models.MessageFlags{Value: event.Flags.Value}

			if uid == currentID {
				userEvent.Flags.Add(lp_models.FlagOutbox)
			} else {
				userEvent.Flags.Add(lp_models.FlagUnread)
			}

			_, _, err := r.LPRepo.PushEvent(ctx, uid, "new_msg", userEvent)
			if err == nil {
				r.Broadcaster.Notify(uid)
			}
		}
	}(recipients, lpEvent, currentUserID)

	c.JSON(http.StatusOK, gin.H{
		"response": finalLocalID,
	})
}
