package redis_repo

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type Repo struct {
	Client *redis.Client
}

func NewRepo(Client *redis.Client) *Repo {
	return &Repo{Client: Client}
}

func (r *Repo) GetUserIDBySession(ctx context.Context, key string) (int64, error) {
	val, err := r.Client.Get(ctx, "im:session:api:"+key).Result()
	if err == redis.Nil {
		return 0, ErrKeyNotFound
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}
