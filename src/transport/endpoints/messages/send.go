package messages

import (
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
		r.Reject(c, 100, "Invalid peer_id")
		return
	}

	// ------------------------------------

	message := c.Query("message")
	attachment := c.Query("attachment")
	randomIDStr := c.Query("random_id")
	replyToStr := c.Query("reply_to")
	forwardMessagesRaw := c.Query("forward_messages")

	// Message n Attachment
	if message == "" && attachment == "" {
		r.Reject(c, 100, "One of the parameters is missing: message or attachment")
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
	redisKey := "rid:" + strconv.FormatInt(currentUserID, 10) + ":" + randomIDStr

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

	if currentUserID < 0 && peerID > 0 && peerID < 2000000000 {
		conv, err := chat.GetConversation(dbx.Instance, peerID)
		if err != nil {
			r.Reject(c, 10, "Internal server error")
			return
		}

		if conv == nil {
			r.Reject(c, 901, "Can't send messages for users without permission")
			return
		}
	}

	if peerID > 2000000000 {
		inChat, err := chat.IsUserInChat(nil, peerID, currentUserID)
		if err != nil || !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	// ------------------------------------------------

	var finalLocalID uint64
	err = dbx.Instance.Transaction(func(tx *gorm.DB) error {
		localID, err := chat.NextLocalID(tx, peerID, currentUserID)
		if err != nil {
			return err
		}
		finalLocalID = localID

		if peerID < 2000000000 {
			var member db_models.ConversationMember
			res := tx.Where("peer_id = ? AND user_id = ?", peerID, currentUserID).First(&member)
			if res.Error == gorm.ErrRecordNotFound {
				tx.Create(&db_models.ConversationMember{
					PeerID:   peerID,
					UserID:   currentUserID,
					JoinedAt: time.Now(),
					IsAdmin:  true,
				})

				if peerID != currentUserID {
					tx.Create(&db_models.ConversationMember{
						PeerID:   peerID,
						UserID:   peerID,
						JoinedAt: time.Now(),
					})
				}
			}
		}

		newMessage := db_models.Message{
			PeerID:          peerID,
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

		if message != "" {
			indexes := r.SearchRepo.GenerateBlindIndexes(newMessage.ID, peerID, message)
			if len(indexes) > 0 {
				if err := tx.Create(&indexes).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		r.Reject(c, 10, "Internal server error during saving")
		return
	}

	r.LPRepo.Client.Set(c.Request.Context(), redisKey, finalLocalID, 24*time.Hour)

	lpEvent := lp_models.NewMessageEvent{
		MessageID:   finalLocalID,
		Flags:       lp_models.MessageFlags{Value: 0},
		PeerID:      peerID,
		Timestamp:   int(time.Now().Unix()),
		Text:        message,
		Attachments: make(map[string]interface{}),
	}

	var recipients []int64
	if peerID > 2000000000 {
		dbx.Instance.Model(&db_models.ConversationMember{}).
			Where("peer_id = ?", peerID).
			Pluck("user_id", &recipients)
	} else {
		recipients = append(recipients, currentUserID)
		if peerID != currentUserID {
			recipients = append(recipients, peerID)
		}
	}

	for _, uid := range recipients {
		userEvent := lpEvent
		if uid == currentUserID {
			userEvent.Flags.Add(lp_models.FlagOutbox)
		} else {
			userEvent.Flags.Add(lp_models.FlagUnread)
		}

		_, _, err := r.LPRepo.PushEvent(c.Request.Context(), uid, "new_msg", userEvent)
		if err == nil {
			r.Broadcaster.Notify(uid)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response": finalLocalID,
	})
}
