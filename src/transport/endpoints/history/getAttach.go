package history

import (
	"net/http"
	"strconv"

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
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID > 2000000000 {
		inChat, _ := chat.IsUserInChat(nil, chatID, currentUserID)
		if !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	mediaType := c.DefaultQuery("media_type", "photo")
	count, _ := strconv.Atoi(c.DefaultQuery("count", "30"))
	if count > 200 {
		count = 200
	}

	startFrom := c.Query("start_from")
	offset, _ := strconv.Atoi(startFrom)

	var msgs []db_models.Message

	query := db.Instance.Where("chat_id = ? AND attachments != '[]' AND attachments IS NOT NULL", chatID)

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
	messagesProcessed := 0

	for _, m := range msgs {
		messagesProcessed++
		var attachments []map[string]interface{}
		if err := m.Attachments.Unmarshal(&attachments); err != nil {
			continue
		}

		for _, att := range attachments {
			if att["type"] == mediaType {
				historyItems = append(historyItems, db_models.VKApiHistoryAttachment{
					MessageID:  m.LocalID,
					Attachment: att,
				})
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

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"items":     historyItems,
			"next_from": nextFrom,
		},
	})
}
