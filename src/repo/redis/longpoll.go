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

var eventRegistry = map[string]func() lp_models.VKEvent{
	// Message events
	"new_msg":           func() lp_models.VKEvent { return &lp_models.NewMessageEvent{} },
	"msg_update":        func() lp_models.VKEvent { return &lp_models.UpdateMessageEvent{} },
	"msg_delete":        func() lp_models.VKEvent { return &lp_models.MsgDeleteEvent{} },
	"msg_replace_flags": func() lp_models.VKEvent { return &lp_models.MsgReplaceFlagsEvent{} },
	"msg_set_flags":     func() lp_models.VKEvent { return &lp_models.MsgSetFlagsEvent{} },
	"msg_reset_flags":   func() lp_models.VKEvent { return &lp_models.MsgResetFlagsEvent{} },
	"mass_delete":       func() lp_models.VKEvent { return &lp_models.MassDeleteMessagesEvent{} },
	"mass_restore":      func() lp_models.VKEvent { return &lp_models.MassRestoreMessagesEvent{} },

	// Read events
	"read_income_before":  func() lp_models.VKEvent { return &lp_models.ReadIncomeBeforeEvent{} },
	"read_outcome_before": func() lp_models.VKEvent { return &lp_models.ReadOutcomeBeforeEvent{} },

	// User status events
	"got_online":  func() lp_models.VKEvent { return &lp_models.GotOnlineEvent{} },
	"got_offline": func() lp_models.VKEvent { return &lp_models.GotOfflineEvent{} },

	// Chat flags events
	"chat_reset_flags":   func() lp_models.VKEvent { return &lp_models.ChatResetFlagsEvent{} },
	"chat_replace_flags": func() lp_models.VKEvent { return &lp_models.ChatReplaceFlagsEvent{} },
	"chat_set_flags":     func() lp_models.VKEvent { return &lp_models.ChatSetFlagsEvent{} },

	// Sync events (v3)
	"state_sync":    func() lp_models.VKEvent { return &lp_models.StateSyncEvent{} },
	"metadata_sync": func() lp_models.VKEvent { return &lp_models.MetaDataSyncEvent{} },

	// Chat events
	"chat_something_changed": func() lp_models.VKEvent { return &lp_models.ChatSomethingChangedEvent{} },
	"chat_update":            func() lp_models.VKEvent { return &lp_models.ChatUpdateEvent{} },

	// Typing events
	"is_dm_typing":   func() lp_models.VKEvent { return &lp_models.IsDMTypingEvent{} },
	"is_chat_typing": func() lp_models.VKEvent { return &lp_models.IsChatTypingEvent{} },
	"multi_typing":   func() lp_models.VKEvent { return &lp_models.MultiUsersTypingEvent{} },
	"multi_audio":    func() lp_models.VKEvent { return &lp_models.MultiUsersAudioRecordingEvent{} },

	// Call events
	"call": func() lp_models.VKEvent { return &lp_models.MakingACallEvent{} },

	// etc
	"counter":      func() lp_models.VKEvent { return &lp_models.CounterUpdateEvent{} },
	"notification": func() lp_models.VKEvent { return &lp_models.NotificationSetEvent{} },
}

type StoredEvent struct {
	Type string          `json:"t"`
	Data json.RawMessage `json:"d"`
}

func (r *Repo) GetUserIDByKey(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return 0, errors.New("empty key")
	}

	val, err := r.Client.Get(ctx, "im:lp:key:"+key).Result()
	if err == redis.Nil {
		return 0, ErrKeyNotFound
	}
	userID, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *Repo) GetUserTS(ctx context.Context, userID int64) (uint64, error) {
	val, err := r.Client.Get(ctx, fmt.Sprintf("im:lp:ts:%d", userID)).Uint64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (r *Repo) SetUserTS(ctx context.Context, userID int64, ts uint64) error {
	key := fmt.Sprintf("im:lp:ts:%d", userID)
	return r.Client.Set(ctx, key, ts, 24*time.Hour).Err()
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

		if factory, ok := eventRegistry[stored.Type]; ok {
			ev := factory()
			if err := json.Unmarshal(stored.Data, ev); err == nil {
				events = append(events, ev)
			}
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
