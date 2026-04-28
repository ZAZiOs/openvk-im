package endpoints

import (
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

/*
Логика эндпоинта:

Если передан user_id || chat_id - тупо сделать его peer_id

Есть ли запись о диалоге между отправителем и peerом
Если диалога нет - сделать диалог (CreateConverstaion, UserID1, PeerID2 (can be club)) и сразу же сохранить сообщение
В ином случае добавить сообщение.

*/

func (r *Router) MessagesSend(c *gin.Context) {
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

	message := c.Query("message")
	attachment := c.Query("attachment")

	if message == "" && attachment == "" {
		r.Reject(c, 100, "One of the parameters is missing: message or attachment")
		return
	}

	if len(message) > 9000 {
		r.Reject(c, 914, "Message is too long")
		return
	}

	if attachment != "" {
		if !IsValidAttachments(attachment) {
			r.Reject(c, 100, "Invalid attachment format")
			return
		}
	}

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

	var finalLocalID uint64
	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
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
			ChatID:      peerID,
			LocalID:     localID,
			FromID:      currentUserID,
			Text:        db_models.EncryptedJSON(message),
			Attachments: db_models.EncryptedJSON(attachment),
			CreatedAt:   time.Now(),
		}

		return tx.Create(&newMessage).Error
	})

	if err != nil {
		r.Reject(c, 10, "Internal server error during saving")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": finalLocalID,
	})
	// Добавить longpoll event что пришло сообщение.

}
