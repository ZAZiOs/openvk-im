package lp_ep

import (
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	redis_repo "ovk-im/src/repo/redis"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

/*
messages.getLongPollHistory

Метод позволяет получить историю событий LongPoll для клиента, который был оффлайн.
Используется для синхронизации состояния, когда TS в LongPoll сервере устарел (ошибка 4)
или когда клиент только что запустился.

Параметры:
- ts (uint64): Последний известный TS.
- pts (uint64): (необязательно) Последний известный PTS.
- version (int): Версия LongPoll (по умолчанию 2).
- events_limit (int): Максимум событий в ответе (default 1000).
- msgs_limit (int): Максимум новых сообщений в ответе (default 200).

Результат:
- history: [][]interface{} (массив событий в формате LongPoll).
- messages: {count: int, items: []VKApiMessage} (полные объекты сообщений).
- profiles: []int64 (ID встретившихся пользователей).
- groups: []int64 (ID встретившихся сообществ).
- new_pts: uint64 (актуальный PTS для последующих запросов).
- new_ts: uint64 (актуальный TS).
*/

func GetLongPollHistory(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)

	ts, _ := strconv.ParseUint(c.Query("ts"), 10, 64)
	pts, _ := strconv.ParseUint(c.Query("pts"), 10, 64)
	eventsLimit, _ := strconv.Atoi(c.DefaultQuery("events_limit", "1000"))
	msgsLimit, _ := strconv.Atoi(c.DefaultQuery("msgs_limit", "200"))
	if msgsLimit > 1000 {
		msgsLimit = 1000
	}

	version, _ := strconv.Atoi(c.DefaultQuery("version", "2"))
	if v := c.Query("lp_version"); v != "" {
		if ver, err := strconv.Atoi(v); err == nil {
			version = ver
		}
	}
	mode, _ := strconv.Atoi(c.DefaultQuery("mode", "2"))

	lpCfg := lp_models.LPConfig{
		Version:   version,
		Mode:      mode,
		Described: 0,
	}

	ctx := c.Request.Context()
	if ts == 0 {
		currentTS, _ := r.LPRepo.GetUserTS(ctx, userID)
		ts = currentTS
	}

	// Получаем сырые события из Redis или БД событий
	rawEvents, newTS, err := r.LPRepo.GetUpdates(ctx, userID, ts)
	if err != nil && err != redis_repo.ErrTsTooOld {
		// If error is other than outdated, reject
		r.Reject(c, 10, "Internal server error: "+err.Error())
		return
	}
	if err == redis_repo.ErrTsTooOld && pts == 0 {
		r.Reject(c, 907, "History outdated")
		return
	}

	history := make([]interface{}, 0)
	msgItems := make([]db_models.VKApiMessage, 0)

	for _, ev := range rawEvents {
		if len(history) >= eventsLimit {
			break
		}

		sliceData := ev.ToSlice(lpCfg) // Обычно используем v3
		slice, ok := sliceData.([]interface{})
		if !ok || len(slice) == 0 {
			continue
		}

		history = append(history, slice) // Добавляем событие в историю
		code := slice[0].(int)

		// Если это новое сообщение (код 4)
		if code == 4 && len(msgItems) < msgsLimit {
			localID := slice[1].(uint64)
			peerID := slice[3].(int64)

			// МАГИЯ: Получаем internal_id, чтобы найти сообщение в БД
			chatID := chat.GetInternalChatID(peerID, userID)

			var m db_models.Message
			q := db.Instance.Where("chat_id = ? AND local_id = ?", chatID, localID)
			q = db_models.BuildVisibilityFilter(q, chatID, userID)
			err := q.First(&m).Error
			if err == nil {
				msgItems = append(msgItems, m.ToVKApiStruct(db.Instance, 0, userID, peerID))
			}
		}
	}

	var userIDs, groupIDs []int64
	core.CollectAllEntityIDs(msgItems, &userIDs, &groupIDs)

	newPTS, _ := r.LPRepo.GetUserPTS(ctx, userID)

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"history":  history,
			"messages": gin.H{"count": len(msgItems), "items": msgItems},
			"profiles": userIDs,
			"groups":   groupIDs,
			"new_pts":  newPTS,
			"new_ts":   newTS,
		},
	})
}
