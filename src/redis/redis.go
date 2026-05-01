package redis

import (
	"context"
	"fmt"
	"log"
	env "ovk-im/src/config"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	Client *redis.Client
	Ctx    = context.Background()
)

func Init() {
	host := env.Get("REDIS_HOST", "127.0.0.1")
	port := env.Get("REDIS_PORT", "6379")
	pass := env.Get("REDIS_PASS", "")
	db := env.Get("REDIS_DB", "0")
	dbn, _ := strconv.Atoi(db)

	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: pass,
		DB:       dbn,
		PoolSize: 50,
	})

	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	if _, err := Client.Ping(ctx).Result(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	log.Printf("Redis connected to %s:%s (db: %d)", host, port, dbn)
}
