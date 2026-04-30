package longpoll_transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	longpoll "ovk-im/src/repo/redis"
	"ovk-im/src/transport/broadcaster"
)

func LongPollHandler(
	w http.ResponseWriter,
	r *http.Request,
	b *broadcaster.Broadcaster,
	lpRepo *longpoll.Repo,
) {
	query := r.URL.Query()
	if query.Get("health") != "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	key := query.Get("key")
	if key == "" {
		json.NewEncoder(w).Encode(lp_models.Envelope{Failed: 2})
		return
	}

	ts, err := strconv.ParseUint(query.Get("ts"), 10, 64)
	if err != nil {
		json.NewEncoder(w).Encode(lp_models.Envelope{Failed: 1})
		return
	}
	mode, _ := strconv.Atoi(query.Get("mode"))
	versionStr := query.Get("version")

	if versionStr == "" {
		versionStr = "2"
	}
	version, _ := strconv.Atoi(versionStr)

	wait, _ := strconv.Atoi(query.Get("wait"))
	if wait <= 5 {
		wait = 30
	} else if wait > 90 {
		wait = 90
	}

	config := lp_models.LPConfig{
		Version: version,
		Mode:    mode,
	}

	subjectID, err := lpRepo.GetUserIDByKey(r.Context(), key)
	if err != nil {
		json.NewEncoder(w).Encode(lp_models.Envelope{Failed: 2})
		return
	}

	packUpdates := func(events []lp_models.VKEvent) []json.RawMessage {
		res := make([]json.RawMessage, 0, len(events))
		for _, ev := range events {
			if _, ok := ev.(lp_models.MakingACallEvent); ok && !config.HasExtended() {
				continue
			}

			slice := ev.ToSlice(config)
			data, _ := json.Marshal(slice)
			res = append(res, json.RawMessage(data))
		}
		return res
	}

	notify := b.Subscribe(subjectID)
	defer b.Unsubscribe(subjectID, notify)

	updates, newTS, err := lpRepo.GetUpdates(r.Context(), subjectID, ts)

	if err == longpoll.ErrTsTooOld {
		json.NewEncoder(w).Encode(lp_models.Envelope{
			Failed: 1,
			TS:     newTS,
		})
		return
	}

	if len(updates) > 0 {
		json.NewEncoder(w).Encode(lp_models.Envelope{
			TS:      newTS,
			Updates: packUpdates(updates),
			PTS:     mapPts(r.Context(), subjectID, config),
		})
		return
	}

	select {
	case <-notify:
		freshUpdates, latestTS, _ := lpRepo.GetUpdates(r.Context(), subjectID, ts)
		json.NewEncoder(w).Encode(lp_models.Envelope{
			TS:      latestTS,
			Updates: packUpdates(freshUpdates),
		})

	case <-time.After(time.Duration(wait) * time.Second):
		currentTS, _ := lpRepo.GetUserTS(r.Context(), subjectID)
		if currentTS == 0 {
			currentTS = ts
		}

		json.NewEncoder(w).Encode(lp_models.Envelope{
			TS:      currentTS,
			Updates: []json.RawMessage{},
		})

	case <-r.Context().Done():
		return
	}
}

func mapPts(ctx context.Context, userID int64, cfg lp_models.LPConfig) uint64 {
	if !cfg.HasPTS() {
		return 0
	}

	var state db_models.ImState
	err := dbx.Instance.WithContext(ctx).First(&state, "user_id = ?", userID).Error
	if err != nil {
		return 0
	}

	return state.PTS
}
