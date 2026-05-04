package custom

import (
	"net/http"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
)

func GetMe(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	userID := val.(int64)

	c.JSON(http.StatusOK, gin.H{
		"response": userID,
	})
}
