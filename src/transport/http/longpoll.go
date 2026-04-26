package http_transport

import (
	"encoding/json"
	"net/http"
	"ovk-im/src/db/repo"
	"ovk-im/src/transport/broadcaster"
	"strconv"
	"time"
)

func LongPollHandler(w http.ResponseWriter, r *http.Request, userID int64) {
	tsStr := r.URL.Query().Get("ts")
	ts, _ := strconv.ParseUint(tsStr, 10, 64)

	// 1. Сначала быстро проверяем БД: вдруг пока юзер переподключался, что-то пришло?
	// Напиши в repo метод GetUpdates(userID, ts)
	updates, _ := repo.GetUpdatesForUser(userID, ts)
	if len(updates) > 0 {
		json.NewEncoder(w).Encode(updates)
		return
	}
	notify := make(chan struct{}, 1)

	broadcaster.Subscribe(userID, notify)
	defer broadcaster.Unsubscribe(userID, notify)

	select {
	case <-notify:
		newUpdates, _ := repo.GetUpdatesForUser(userID, ts)
		json.NewEncoder(w).Encode(newUpdates)
	case <-time.After(25 * time.Second):
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updates":[], "ts":` + tsStr + `}`))
	}
}
