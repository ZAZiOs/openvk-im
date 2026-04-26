package http_transport

import (
	"net/http"
	"ovk-im/src/redis"
)

func AuthMiddleware(next func(w http.ResponseWriter, r *http.Request, userID int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, `{"error":"no_key"}`, http.StatusForbidden)
			return
		}

		userID, err := redis.Client.Get(redis.Ctx, redis.Key("lp_key:"+key)).Int64()
		if err != nil {
			http.Error(w, `{"error":"invalid_key"}`, http.StatusUnauthorized)
			return
		}
		next(w, r, userID)
	}
}
