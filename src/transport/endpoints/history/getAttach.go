package history

import (
	"net/http"
	"strconv"
	"strings"

	"ovk-im/src/db"

	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

/*
messages.getHistoryAttachments

Возвращает список медиа-файлов из диалога.

Параметры:
- peer_id (int, required): ID диалога/чата.
- media_type (string, default "photo"): Тип вложений (photo, video, audio, doc, link).
- start_from (string, optional): Смещение для пагинации (берется из next_from предыдущего ответа).
- count (int, max 200, default 30): Кол-во объектов.
- preserve_order (flag 0/1): 1 — хронологический порядок, 0 — обратный.

Результат:
- items (array): Список объектов {message_id, attachment: {type, photo/doc/...}}
- next_from (string): Значение для следующего запроса.
*/

func GetHistoryAttachments(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	uIDParam, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	chatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)

	if chatID == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	if peerID == 0 {
		peerID = chat.DerivePeerID(chatID, currentUserID)
	}

	isGroupChat := strings.HasPrefix(chatID, "c")

	var member *db_models.ConversationMember
	if currentUserID != 0 {
		if isGroupChat {
			var err error
			member, err = chat.GetMember(db.Instance, chatID, currentUserID)
			if err != nil || member == nil || member.LeftAt != nil {
				r.Reject(c, 917, "You don't have access to this chat")
				return
			}
		} else {
			member, _ = chat.GetMember(db.Instance, chatID, currentUserID)
			if member == nil {
				r.Reject(c, 917, "Conversation doesn't exist")
				return
			}
		}
	}


	mediaType := strings.ToLower(c.DefaultQuery("media_type", "photo"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "30"))
	if count > 200 {
		count = 200
	} else if count < 1 {
		count = 30
	}

	startFrom := c.Query("start_from")
	offset, _ := strconv.Atoi(startFrom)

	var msgs []db_models.Message
	query := db.Instance.Where("chat_id = ? AND attachments != '[]' AND attachments IS NOT NULL", chatID)
	query = db_models.BuildVisibilityFilter(query, chatID, currentUserID)

	order := "local_id DESC"
	if c.Query("preserve_order") == "1" {
		order = "local_id ASC"
	}

	fetchLimit := count * 5
	if fetchLimit > 500 {
		fetchLimit = 500
	}

	err := query.Order(order).Limit(fetchLimit).Offset(offset).Find(&msgs).Error
	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	var historyItems []db_models.VKApiHistoryAttachment
	var userIDs, groupIDs []int64
	messagesProcessed := 0

	for _, m := range msgs {
		messagesProcessed++
		var attachments []map[string]interface{}
		if err := m.Attachments.Unmarshal(&attachments); err != nil {
			continue
		}

		for _, att := range attachments {
			attType, _ := att["type"].(string)
			matches := attType == mediaType
			if !matches {
				if mediaType == "link" && (attType == "share" || attType == "link") {
					matches = true
				} else if mediaType == "doc" && (attType == "doc" || attType == "graffiti") {
					matches = true
				} else if mediaType == "audio_message" && (attType == "audio_message" || attType == "audiomessage") {
					matches = true
				}
			}

			if matches {
				msgID := m.ID
				if msgID == 0 {
					msgID = m.LocalID
				}

				historyItems = append(historyItems, db_models.VKApiHistoryAttachment{
					MessageID:  msgID,
					Attachment: att,
				})

				if m.FromID > 0 {
					userIDs = append(userIDs, m.FromID)
				} else if m.FromID < 0 {
					groupIDs = append(groupIDs, -m.FromID)
				}
			}
		}

		if len(historyItems) >= count {
			break
		}
	}

	nextFrom := ""
	if len(msgs) > 0 && messagesProcessed > 0 {
		nextFrom = strconv.Itoa(offset + messagesProcessed)
	}
	if len(msgs) < fetchLimit && len(historyItems) < count {
		nextFrom = ""
	}

	response := gin.H{
		"items":     historyItems,
		"next_from": nextFrom,
	}

	if c.Query("extended") == "1" || c.Query("fields") != "" {
		response["profiles"] = core.UniqueIDs(userIDs)
		response["groups"] = core.UniqueIDs(groupIDs)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
