package lp_ep

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"time"

	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetLongPollServer(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)

	keyData := fmt.Sprintf("%d_%d_%d", userID, time.Now().UnixNano(), 42)
	lpKey := fmt.Sprintf("%x", md5.Sum([]byte(keyData)))

	ctx := c.Request.Context()
	err := r.LPRepo.Client.Set(ctx, "im:lp:key:"+lpKey, userID, 12*time.Hour).Err()
	if err != nil {
		r.Reject(c, 10, "Failed to create LP session")
		return
	}

	ts, _ := r.LPRepo.GetUserTS(ctx, userID)
	pts, _ := r.LPRepo.GetUserPTS(ctx, userID)

	if ts == 0 {
		ts = uint64(time.Now().Unix())
		r.LPRepo.Client.Set(ctx, fmt.Sprintf("im:lp:ts:%d", userID), ts, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"key":    lpKey,
			"server": "%DOMAIN%/nim",
			"ts":     ts,
			"pts":    pts,
		},
	})
}
