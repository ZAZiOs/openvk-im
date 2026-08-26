package messages

import (
	"net/http"
	"strconv"
	"time"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

/*
Gemini: (Потому что мне лень самому писать документацию)
messages.search

Возвращает список сообщений, найденных по заданным критериям поиска (используется Blind Indexing для поиска по зашифрованным данным).

Параметры (Query):
- q (string, required): Подстрока, по которой будет производиться поиск.
- peer_id (int, optional): Идентификатор назначения (диалога). Если не передан, поиск ведется по всем доступным чатам.
- date (string, optional): Дата в формате DDMMYYYY. Будут возвращены сообщения, отправленные до этой даты.
- preview_length (int, optional, default 0): Количество символов для обрезки текста сообщения (0 — без обрезки).
- count (int, optional, default 20, max 100): Количество сообщений, которое необходимо получить.
- offset (int, optional, default 0): Смещение для выборки.
- extended (flag 0/1, optional): Если 1, возвращает дополнительные массивы идентификаторов для профилей и сообществ.

Результат:
Объект response, содержащий:
- count (int): Общее количество найденных сообщений (без учета пагинации).
- items (array): Массив объектов сообщений (VKApiMessage).
- profiles (array, optional): Массив идентификаторов (int64) пользователей, фигурирующих в результатах (только при extended=1).
- groups (array, optional): Массив идентификаторов (int64) сообществ, фигурирующих в результатах (только при extended=1).

Особенности:
1. Поиск работает по полному совпадению слов (из-за ограничений шифрования). Частичные совпадения ("прив" для "привет") не поддерживаются.
2. При поиске без peer_id автоматически учитываются только те чаты, в которых состоит текущий пользователь.
*/

func Search(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	q := c.Query("q")
	if q == "" {
		r.Reject(c, 100, "One of the parameters is missing: q")
		return
	}

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 100 {
		count = 100
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	previewLen, _ := strconv.Atoi(c.DefaultQuery("preview_length", "0"))

	var messageIDs []uint64
	var err error

	uIDParam, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	targetChatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)

	if targetChatID != "" {
		messageIDs, err = r.SearchRepo.SearchMessages(targetChatID, q)
	} else if currentUserID == 0 {
		err = db.Instance.Model(&db_models.MessageSearchIndex{}).
			Select("message_id").
			Where("word_hash IN ?", r.SearchRepo.PrepareHashes(q)).
			Group("message_id").
			Having("COUNT(DISTINCT word_hash) = ?", r.SearchRepo.WordsCount(q)).
			Pluck("message_id", &messageIDs).Error
	} else {
		var myChatIDs []string
		db.Instance.Model(&db_models.ConversationMember{}).
			Where("user_id = ?", currentUserID).
			Pluck("internal_chat_id", &myChatIDs)

		if len(myChatIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"response": gin.H{"count": 0, "items": []interface{}{}}})
			return
		}

		err = db.Instance.Model(&db_models.MessageSearchIndex{}).
			Select("message_id").
			Where("chat_id IN ? AND word_hash IN ?", myChatIDs, r.SearchRepo.PrepareHashes(q)).
			Group("message_id").
			Having("COUNT(DISTINCT word_hash) = ?", r.SearchRepo.WordsCount(q)).
			Pluck("message_id", &messageIDs).Error
	}

	if err != nil || len(messageIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"response": gin.H{"count": 0, "items": []interface{}{}}})
		return
	}


	var msgs []db_models.Message
	query := db.Instance.Where("id IN ?", messageIDs)
	query = db_models.BuildVisibilityFilter(query, targetChatID, currentUserID)

	if dateParam := c.Query("date"); dateParam != "" {
		if t, err := time.Parse("02012006", dateParam); err == nil {
			query = query.Where("created_at < ?", t)
		}
	}

	err = query.Order("created_at DESC").Limit(count).Offset(offset).Find(&msgs).Error
	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	extended := c.Query("extended") == "1"
	userIDsMap := make(map[int64]bool)
	groupIDsMap := make(map[int64]bool)

	responseItems := make([]db_models.VKApiMessage, len(msgs))
	for i, m := range msgs {
		msgPeerID := chat.DerivePeerID(m.ChatID, currentUserID)
		v := m.ToVKApiStruct(db.Instance, 0, currentUserID, msgPeerID)

		if previewLen > 0 && len(v.Text) > previewLen {
			v.Text = v.Text[:previewLen] + "…"
		}
		responseItems[i] = v

		if extended {
			if m.FromID > 0 {
				userIDsMap[m.FromID] = true
			} else if m.FromID < 0 {
				groupIDsMap[-m.FromID] = true
			}
			if msgPeerID > 0 && msgPeerID < 2000000000 {
				userIDsMap[msgPeerID] = true
			} else if msgPeerID < 0 {
				groupIDsMap[-msgPeerID] = true
			}

			if m.ActionMid != 0 {
				if m.ActionMid > 0 {
					userIDsMap[m.ActionMid] = true
				} else {
					groupIDsMap[-m.ActionMid] = true
				}
			}
		}
	}

	result := gin.H{
		"count": len(messageIDs),
		"items": responseItems,
	}

	if extended {
		userIDs := []int64{}
		for id := range userIDsMap {
			userIDs = append(userIDs, id)
		}
		groupIDs := []int64{}
		for id := range groupIDsMap {
			groupIDs = append(groupIDs, id)
		}

		result["profiles"] = userIDs
		result["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": result,
	})
}
