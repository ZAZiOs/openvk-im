package redis_repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm/clause"
)

var (
	ErrTsTooOld    = errors.New("history outdated")
	ErrKeyNotFound = errors.New("key expired or invalid")
)

type StoredEvent struct {
	Type string          `json:"t"`
	Data json.RawMessage `json:"d"`
}

func (r *Repo) GetUserIDByKey(ctx context.Context, key string) (int64, error) {
	val, err := r.Client.Get(ctx, "im:lp:key:"+key).Result()
	if err == redis.Nil {
		return 0, ErrKeyNotFound
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (r *Repo) GetUserTS(ctx context.Context, userID int64) (uint64, error) {
	val, err := r.Client.Get(ctx, fmt.Sprintf("im:lp:ts:%d", userID)).Uint64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (r *Repo) GetUpdates(ctx context.Context, userID int64, lastTS uint64) ([]lp_models.VKEvent, uint64, error) {
	key := fmt.Sprintf("im:lp:events:%d", userID)

	first, err := r.Client.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err == nil && len(first) > 0 {
		if float64(lastTS) < first[0].Score-1 {
			currentTS, _ := r.GetUserTS(ctx, userID)
			return nil, currentTS, ErrTsTooOld
		}
	}

	res, err := r.Client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     key,
		ByScore: true,
		Start:   fmt.Sprintf("(%d", lastTS),
		Stop:    "+inf",
	}).Result()

	if err != nil {
		return nil, lastTS, err
	}

	events := make([]lp_models.VKEvent, 0, len(res))
	for _, v := range res {
		var stored StoredEvent
		if err := json.Unmarshal([]byte(v), &stored); err != nil {
			continue
		}

		var ev lp_models.VKEvent

		// Маппинг строковых типов в структуры lpm
		switch stored.Type {
		case "msg_new":
			var e lp_models.NewMessageEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "msg_delete":
			var e lp_models.MsgDeleteEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "msg_flags_replace":
			var e lp_models.MsgReplaceFlagsEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "msg_flags_set":
			var e lp_models.MsgSetFlagsEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "msg_flags_reset":
			var e lp_models.MsgResetFlagsEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "read_in":
			var e lp_models.ReadIncomeBeforeEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "read_out":
			var e lp_models.ReadOutcomeBeforeEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "online":
			var e lp_models.GotOnlineEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "offline":
			var e lp_models.GotOfflineEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "typing_dm":
			var e lp_models.IsDMTypingEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		case "typing_chat":
			var e lp_models.IsChatTypingEvent
			json.Unmarshal(stored.Data, &e)
			ev = e
		}

		if ev != nil {
			events = append(events, ev)
		}
	}

	newTS, _ := r.GetUserTS(ctx, userID)
	if newTS < lastTS {
		newTS = lastTS
	}

	return events, newTS, nil
}

func (r *Repo) PushEvent(ctx context.Context, userID int64, eventType string, event lp_models.VKEvent) (uint64, uint64, error) {
	tsKey := fmt.Sprintf("im:lp:ts:%d", userID)
	newTS, _ := r.Client.Incr(ctx, tsKey).Result()

	var newPTS uint64
	if isHistoryEvent(eventType) {
		p, err := r.incrementUserPTS(ctx, userID)
		if err == nil {
			newPTS = p
		}
	} else {
		newPTS, _ = r.GetUserPTS(ctx, userID)
	}

	payload, _ := json.Marshal(event)
	stored, _ := json.Marshal(StoredEvent{
		Type: eventType,
		Data: payload,
	})
	eventsKey := fmt.Sprintf("im:lp:events:%d", userID)

	pipe := r.Client.Pipeline()
	pipe.ZAdd(ctx, eventsKey, redis.Z{Score: float64(newTS), Member: stored})
	pipe.ZRemRangeByRank(ctx, eventsKey, 0, -201)
	pipe.Expire(ctx, eventsKey, 48*time.Hour)

	_, err := pipe.Exec(ctx)
	return uint64(newTS), newPTS, err
}

func (r *Repo) incrementUserPTS(ctx context.Context, userID int64) (uint64, error) {
	ptsKey := fmt.Sprintf("im:lp:pts:%d", userID)

	newPTS, err := r.Client.Incr(ctx, ptsKey).Result()
	if err != nil {
		return 0, err
	}
	err = dbx.Instance.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"pts": uint64(newPTS)}),
	}).Create(&db_models.ImState{
		UserID: userID,
		PTS:    uint64(newPTS),
	}).Error

	if err != nil {
		fmt.Printf("failed to sync pts to db for user %d: %v\n", userID, err)
	}

	return uint64(newPTS), nil
}

func (r *Repo) GetUserPTS(ctx context.Context, userID int64) (uint64, error) {
	ptsKey := fmt.Sprintf("im:lp:pts:%d", userID)

	val, err := r.Client.Get(ctx, ptsKey).Uint64()
	if err == nil {
		return val, nil
	}

	if err == redis.Nil {
		var state db_models.ImState
		err = dbx.Instance.WithContext(ctx).First(&state, "user_id = ?", userID).Error

		if err != nil {
			return 0, nil
		}

		r.Client.Set(ctx, ptsKey, state.PTS, 0)
		return state.PTS, nil
	}

	return 0, err
}

func isHistoryEvent(t string) bool {
	switch t {
	case "msg_new", "msg_delete", "msg_flags_replace", "read_in", "read_out":
		return true
	}
	return false
}
