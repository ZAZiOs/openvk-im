package custom

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
)

func SendAction(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)


	var peerID int64
	if pID := c.DefaultQuery("peer_id", c.PostForm("peer_id")); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if uID := c.DefaultQuery("user_id", c.PostForm("user_id")); uID != "" {
		id, _ := strconv.ParseInt(uID, 10, 64)
		peerID = id
	} else if chatID := c.DefaultQuery("chat_id", c.PostForm("chat_id")); chatID != "" {
		id, _ := strconv.ParseInt(chatID, 10, 64)
		peerID = 2000000000 + id
	}

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id, user_id or chat_id")
		return
	}

	actionType := c.DefaultQuery("action_type", c.PostForm("action_type"))
	if actionType == "" {
		r.Reject(c, 100, "Parameter missing: action_type")
		return
	}

	var actionMid int64
	if aMid := c.DefaultQuery("action_mid", c.PostForm("action_mid")); aMid != "" {
		actionMid, _ = strconv.ParseInt(aMid, 10, 64)
	}

	actionText := c.DefaultQuery("action_text", c.PostForm("action_text"))
	message := c.DefaultQuery("message", c.PostForm("message"))

	var randomID int64
	if rID := c.DefaultQuery("random_id", c.PostForm("random_id")); rID != "" {
		randomID, _ = strconv.ParseInt(rID, 10, 64)
	} else {
		randomID = rand.Int63n(2147483647)
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)
	isGroupChat := strings.HasPrefix(internalChatID, "c")

	if isGroupChat {
		inChat, err := chat.IsUserInChat(nil, internalChatID, currentUserID)
		if err != nil || !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	var finalLocalID uint64
	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
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
			ChatID:     internalChatID,
			LocalID:    localID,
			FromID:     currentUserID,
			Text:       db_models.EncryptedJSON(message),
			Action:     actionType,
			ActionMid:  actionMid,
			ActionText: actionText,
			RandomID:   randomID,
			CreatedAt:  time.Now(),
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
		r.Reject(c, 10, "Internal server error: "+err.Error())
		return
	}

	lpAttach := lp_models.LPAttachments{
		Source: actionType,
		From:   strconv.FormatInt(currentUserID, 10),
	}
	if actionMid != 0 {
		lpAttach.Mid = strconv.FormatInt(actionMid, 10)
	}

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
