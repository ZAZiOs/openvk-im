package lp_ep

import (
	"net/http"
	"strconv"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
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

	lpVersion, _ := strconv.Atoi(c.DefaultQuery("version", "2"))
	cfg := lp_models.LPConfig{
		Version: lpVersion,
		Mode:    2 | 8 | 32 | 64,
	}

	ts, _ := strconv.ParseUint(c.Query("ts"), 10, 64)

	eventsLimit, _ := strconv.Atoi(c.DefaultQuery("events_limit", "1000"))
	msgsLimit, _ := strconv.Atoi(c.DefaultQuery("msgs_limit", "200"))

	ctx := c.Request.Context()
	rawEvents, newTS, err := r.LPRepo.GetUpdates(ctx, userID, ts)

	if err != nil {
		r.Reject(c, 907, "History outdated")
		return
	}

	history := make([][]interface{}, 0)
	msgItems := make([]db_models.VKApiMessage, 0)
	userIDs := make(map[int64]bool)
	groupIDs := make(map[int64]bool)

	for _, ev := range rawEvents {
		if len(history) >= eventsLimit || len(msgItems) >= msgsLimit {
			break
		}

		slice := ev.ToSlice(cfg)
		if len(slice) == 0 {
			continue
		}

		code := slice[0].(int)

		if code == 8 || code == 9 || code == 61 || code == 62 {
			continue
		}

		if code == 4 {
			history = append(history, slice[:3])
			localID := slice[1].(uint64)
			peerID := slice[3].(int64)

			var m db_models.Message
			err := db.Instance.Where("peer_id = ? AND local_id = ?", peerID, localID).First(&m).Error
			if err == nil {
				v := m.ToVKApiStruct(db.Instance, 0, userID, peerID)
				msgItems = append(msgItems, v)

				if v.FromID > 0 {
					userIDs[v.FromID] = true
				} else if v.FromID < 0 {
					groupIDs[-v.FromID] = true
				}
			}
		}
	}

	newPTS, _ := r.LPRepo.GetUserPTS(ctx, userID)

	profiles := []int64{}
	for id := range userIDs {
		profiles = append(profiles, id)
	}

	groups := []int64{}
	for id := range groupIDs {
		groups = append(groups, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"history": history,
			"messages": lp_models.VKApiMessagesWithCount{
				Count: len(msgItems),
				Items: msgItems,
			},
			"profiles": profiles,
			"groups":   groups,
			"new_pts":  newPTS,
			"new_ts":   newTS,
		},
	})
}
