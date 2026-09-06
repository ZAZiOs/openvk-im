package status

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	env "ovk-im/src/config"
	lp_models "ovk-im/src/models/longpoll"
	redis_repo "ovk-im/src/repo/redis"
	"ovk-im/src/transport/broadcaster"
	"ovk-im/src/transport/endpoints/core"
)

func GetOnlineRecipients(ctx context.Context, db *gorm.DB, userID int64) ([]int64, error) {
	openvkDB := env.Get("OPENVK_DB_NAME", "openvk")
	query := fmt.Sprintf(`SELECT s1.target AS recipient_id 
		FROM %s.subscriptions s1 
		JOIN %s.subscriptions s2 ON s1.target = s2.follower AND s1.follower = s2.target 
		WHERE s1.follower = ? AND s1.target != ?
		UNION 
		SELECT cm2.user_id AS recipient_id 
		FROM conversation_members cm1 
		JOIN conversation_members cm2 ON cm1.internal_chat_id = cm2.internal_chat_id 
		WHERE cm1.user_id = ? AND cm2.user_id != ? AND cm1.internal_chat_id LIKE 'u%%%%' AND cm2.left_at IS NULL`,
		openvkDB, openvkDB)

	var recipients []int64
	err := db.WithContext(ctx).Raw(query, userID, userID, userID, userID).Scan(&recipients).Error
	return recipients, err
}

func EmitUserOnline(ctx context.Context, db *gorm.DB, redisClient *redis.Client, lpRepo *redis_repo.Repo, b *broadcaster.Broadcaster, userID int64, extra int64, timestamp uint64) {
	recipients, err := GetOnlineRecipients(ctx, db, userID)
	if err != nil {
		log.Printf("[EmitUserOnline] Failed to get recipients for user %d: %v", userID, err)
		return
	}

	ev := &lp_models.GotOnlineEvent{
		UserID:    userID,
		Extra:     extra,
		Timestamp: timestamp,
	}

	for _, rID := range recipients {
		_ = lpRepo.PushEphemeralEvent(ctx, rID, "got_online", ev)
		if b != nil {
			b.Notify(rID)
		}
		if redisClient != nil {
			_ = redisClient.Publish(ctx, "lp_updates", strconv.FormatInt(rID, 10)).Err()
		}
	}
}

func EmitUserOffline(ctx context.Context, db *gorm.DB, redisClient *redis.Client, lpRepo *redis_repo.Repo, b *broadcaster.Broadcaster, userID int64, flags int64, timestamp uint64) {
	recipients, err := GetOnlineRecipients(ctx, db, userID)
	if err != nil {
		log.Printf("[EmitUserOffline] Failed to get recipients for user %d: %v", userID, err)
		return
	}

	ev := &lp_models.GotOfflineEvent{
		UserID:    userID,
		Flags:     flags,
		Timestamp: timestamp,
	}

	for _, rID := range recipients {
		_ = lpRepo.PushEphemeralEvent(ctx, rID, "got_offline", ev)
		if b != nil {
			b.Notify(rID)
		}
		if redisClient != nil {
			_ = redisClient.Publish(ctx, "lp_updates", strconv.FormatInt(rID, 10)).Err()
		}
	}
}

func TouchUserActivity(ctx context.Context, db *gorm.DB, redisClient *redis.Client, lpRepo *redis_repo.Repo, b *broadcaster.Broadcaster, userID int64) {
	if userID <= 0 {
		return
	}
	now := time.Now().Unix()
	score, err := redisClient.ZScore(ctx, "im:online_users", strconv.FormatInt(userID, 10)).Result()
	wasOnline := err == nil && (now-int64(score) <= 300)

	redisClient.ZAdd(ctx, "im:online_users", redis.Z{
		Score:  float64(now),
		Member: userID,
	})

	if !wasOnline {
		go func(uID int64, ts uint64) {
			EmitUserOnline(context.Background(), db, redisClient, lpRepo, b, uID, 0, ts)
		}(userID, uint64(now))
	}
}

func SetOnline(c *gin.Context, r *core.BaseHandler) {
	var targetUID int64
	if val, ok := c.Get("userID"); ok {
		targetUID = val.(int64)
	}
	if targetUID == 0 {
		if uidStr := c.Query("user_id"); uidStr != "" {
			targetUID, _ = strconv.ParseInt(uidStr, 10, 64)
		}
	}
	if targetUID <= 0 {
		r.Reject(c, 100, "user_id is missing or invalid")
		return
	}

	now := time.Now().Unix()
	score, err := r.LPRepo.Client.ZScore(c.Request.Context(), "im:online_users", strconv.FormatInt(targetUID, 10)).Result()
	wasOnline := err == nil && (now-int64(score) <= 300)

	r.LPRepo.Client.ZAdd(c.Request.Context(), "im:online_users", redis.Z{
		Score:  float64(now),
		Member: targetUID,
	})

	extra, _ := strconv.ParseInt(c.DefaultQuery("extra", "0"), 10, 64)
	force := c.Query("force") == "1"

	if !wasOnline || force {
		go func(uID, ext int64, ts uint64) {
			ctx := context.Background()
			EmitUserOnline(ctx, r.DB, r.LPRepo.Client, r.LPRepo, r.Broadcaster, uID, ext, ts)
		}(targetUID, extra, uint64(now))
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func SetOffline(c *gin.Context, r *core.BaseHandler) {
	var targetUID int64
	if val, ok := c.Get("userID"); ok {
		targetUID = val.(int64)
	}
	if targetUID == 0 {
		if uidStr := c.Query("user_id"); uidStr != "" {
			targetUID, _ = strconv.ParseInt(uidStr, 10, 64)
		}
	}
	if targetUID <= 0 {
		r.Reject(c, 100, "user_id is missing or invalid")
		return
	}

	r.LPRepo.Client.ZRem(c.Request.Context(), "im:online_users", strconv.FormatInt(targetUID, 10))

	flags, _ := strconv.ParseInt(c.DefaultQuery("flags", "0"), 10, 64)
	now := time.Now().Unix()

	go func(uID, fl int64, ts uint64) {
		ctx := context.Background()
		EmitUserOffline(ctx, r.DB, r.LPRepo.Client, r.LPRepo, r.Broadcaster, uID, fl, ts)
	}(targetUID, flags, uint64(now))

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func TouchOnline(c *gin.Context, r *core.BaseHandler) {
	var targetUID int64
	if val, ok := c.Get("userID"); ok {
		targetUID = val.(int64)
	}
	if targetUID == 0 {
		if uidStr := c.Query("user_id"); uidStr != "" {
			targetUID, _ = strconv.ParseInt(uidStr, 10, 64)
		}
	}
	if targetUID <= 0 {
		r.Reject(c, 100, "user_id is missing or invalid")
		return
	}

	now := time.Now().Unix()
	score, err := r.LPRepo.Client.ZScore(c.Request.Context(), "im:online_users", strconv.FormatInt(targetUID, 10)).Result()
	wasOnline := err == nil && (now-int64(score) <= 300)

	r.LPRepo.Client.ZAdd(c.Request.Context(), "im:online_users", redis.Z{
		Score:  float64(now),
		Member: targetUID,
	})

	if !wasOnline {
		extra, _ := strconv.ParseInt(c.DefaultQuery("extra", "0"), 10, 64)
		go func(uID, ext int64, ts uint64) {
			ctx := context.Background()
			EmitUserOnline(ctx, r.DB, r.LPRepo.Client, r.LPRepo, r.Broadcaster, uID, ext, ts)
		}(targetUID, extra, uint64(now))
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func StartOnlineTracker(ctx context.Context, db *gorm.DB, redisClient *redis.Client, lpRepo *redis_repo.Repo, b *broadcaster.Broadcaster) {
	openvkDB := env.Get("OPENVK_DB_NAME", "openvk")
	now := time.Now().Unix()

	var profiles []struct {
		ID     int64 `gorm:"column:id"`
		Online int64 `gorm:"column:online"`
	}
	err := db.WithContext(ctx).Raw(fmt.Sprintf("SELECT id, online FROM %s.profiles WHERE online >= ?", openvkDB), now-300).Scan(&profiles).Error
	if err == nil {
		for _, p := range profiles {
			redisClient.ZAdd(ctx, "im:online_users", redis.Z{
				Score:  float64(p.Online),
				Member: p.ID,
			})
		}
		log.Printf("[OnlineTracker] Seeded %d online users into Redis", len(profiles))
	} else {
		log.Printf("[OnlineTracker] Seed query warning: %v", err)
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkExpiredUsers(ctx, db, redisClient, lpRepo, b, openvkDB)
			}
		}
	}()
}

func checkExpiredUsers(ctx context.Context, db *gorm.DB, redisClient *redis.Client, lpRepo *redis_repo.Repo, b *broadcaster.Broadcaster, openvkDB string) {
	now := time.Now().Unix()
	expiredThreshold := now - 300

	items, err := redisClient.ZRangeByScoreWithScores(ctx, "im:online_users", &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(expiredThreshold, 10),
	}).Result()
	if err != nil || len(items) == 0 {
		return
	}

	for _, item := range items {
		uID, err := strconv.ParseInt(fmt.Sprintf("%v", item.Member), 10, 64)
		if err != nil {
			continue
		}

		lastScore := int64(item.Score)
		redisClient.ZRem(ctx, "im:online_users", item.Member)

		_ = db.Exec(fmt.Sprintf("UPDATE %s.profiles SET online = ? WHERE id = ? AND online < ?", openvkDB), lastScore, uID, lastScore).Error

		EmitUserOffline(ctx, db, redisClient, lpRepo, b, uID, 1, uint64(lastScore))
	}
}
