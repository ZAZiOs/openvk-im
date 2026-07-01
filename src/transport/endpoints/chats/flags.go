package chats

import (
	"net/http"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func MarkAsImportantConversation(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	isImportant := c.Query("important") != "0"

	mode := "reset"
	if isImportant {
		mode = "set"
	}

	err := r.UpdateChatFlags(currentUserID, peerID, 1, mode)
	if err != nil {
		r.Reject(c, 10, "Internal server error or member not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

func MarkAsAnsweredConversation(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	if currentUserID > 0 {
		r.Reject(c, 15, "Access denied: only clubs can mark conversations as answered")
		return
	}

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}

	isAnswered := c.Query("answered") != "0"

	mode := "set"
	if isAnswered {
		mode = "reset"
	}

	err := r.UpdateChatFlags(currentUserID, peerID, 2, mode)
	if err != nil {
		r.Reject(c, 10, "Internal server error or member not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": 1})
}
