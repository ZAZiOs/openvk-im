package lp_ep

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetLongPollServer(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)

	var subjectID int64 = userID
	if gIDStr := c.Query("group_id"); gIDStr != "" {
		groupID, _ := strconv.ParseInt(gIDStr, 10, 64)
		if groupID > 0 {
			subjectID = -groupID
		}
	}

	keyData := fmt.Sprintf("%d_%d_%d", subjectID, time.Now().UnixNano(), 42)
	lpKey := fmt.Sprintf("%x", md5.Sum([]byte(keyData)))

	ctx := c.Request.Context()
	err := r.LPRepo.Client.Set(ctx, "im:lp:key:"+lpKey, subjectID, 12*time.Hour).Err()
	if err != nil {
		r.Reject(c, 10, "Failed to create LP session")
		return
	}

	ts, _ := r.LPRepo.GetUserTS(ctx, subjectID)
	pts, _ := r.LPRepo.GetUserPTS(ctx, subjectID)

	if ts == 0 {
		newTS, incrErr := r.LPRepo.Client.Incr(ctx, fmt.Sprintf("im:lp:ts:%d", subjectID)).Result()
		if incrErr == nil {
			ts = uint64(newTS)
		} else {
			fmt.Printf("Redis error initializing TS: %v\n", incrErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"key":    lpKey,
			"server": "%REPLACE_THIS%",
			"ts":     ts,
			"pts":    pts,
		},
	})
}
