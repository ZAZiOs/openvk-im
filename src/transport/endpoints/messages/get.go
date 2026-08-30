package messages

import (
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Get(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	out, _ := strconv.Atoi(c.DefaultQuery("out", "0"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}
	timeOffset, _ := strconv.Atoi(c.DefaultQuery("time_offset", "0"))
	filters, _ := strconv.Atoi(c.DefaultQuery("filters", "0"))
	lastMessageID, _ := strconv.ParseUint(c.DefaultQuery("last_message_id", "0"), 10, 64)
	previewLength, _ := strconv.Atoi(c.DefaultQuery("preview_length", "0"))

	query := dbx.Instance.Table("messages").
		Joins("JOIN conversation_members ON conversation_members.internal_chat_id = messages.chat_id AND conversation_members.user_id = ? AND conversation_members.left_at IS NULL", currentUserID).
		Where("messages.deleted_at IS NULL").
		Where("messages.local_id > conversation_members.deleted_before_id")

	if out == 1 {
		query = query.Where("messages.from_id = ?", currentUserID)
	} else {
		query = query.Where("messages.from_id != ?", currentUserID)
	}

	if timeOffset > 0 {
		since := time.Now().Add(-time.Duration(timeOffset) * time.Second)
		query = query.Where("messages.created_at >= ?", since)
	}

	if (filters & 4) == 0 {
		if (filters & 1) != 0 {
			query = query.Where("messages.local_id > conversation_members.last_read_id")
		}
		if (filters & 2) != 0 {
			query = query.Where("messages.chat_id NOT LIKE 'chat:%'")
		}
	}

	if lastMessageID > 0 {
		query = query.Where("messages.id < ?", lastMessageID)
	}

	var totalCount int64
	query.Count(&totalCount)

	var dbMessages []db_models.Message
	err := query.Order("messages.created_at DESC, messages.id DESC").
		Limit(count).Offset(offset).
		Find(&dbMessages).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	items := make([]db_models.VKApiMessage, 0, len(dbMessages))
	for _, m := range dbMessages {
		mPeerID := chat.DerivePeerID(m.ChatID, currentUserID)
		vkMsg := m.ToVKApiStruct(dbx.Instance, 5, currentUserID, mPeerID)
		if previewLength > 0 && len([]rune(vkMsg.Text)) > previewLength {
			runes := []rune(vkMsg.Text)
			vkMsg.Text = string(runes[:previewLength]) + "..."
		}
		items = append(items, vkMsg)
	}

	response := gin.H{
		"count": totalCount,
		"items": items,
	}

	if c.Query("extended") == "1" {
		var userIDs []int64
		var groupIDs []int64

		core.CollectAllEntityIDs(items, &userIDs, &groupIDs)

		response["profiles"] = userIDs
		response["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}

func GetByID(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	idsStr := c.Query("message_ids")
	if idsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: message_ids")
		return
	}

	idStrings := strings.Split(idsStr, ",")
	var ids []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		r.Reject(c, 100, "Invalid message_ids")
		return
	}

	var dbMessages []db_models.Message
	var query *gorm.DB
	if currentUserID == 0 {
		query = dbx.Instance.Where("id IN ?", ids)
	} else {
		query = dbx.Instance.Where("id IN ? AND chat_id IN (SELECT internal_chat_id FROM conversation_members WHERE user_id = ? AND left_at IS NULL)",
			ids, currentUserID)
	}
	query = db_models.BuildVisibilityFilter(query, "", currentUserID)
	err := query.Find(&dbMessages).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	items := make([]db_models.VKApiMessage, 0)
	for _, m := range dbMessages {
		mPeerID := chat.DerivePeerID(m.ChatID, currentUserID)
		items = append(items, m.ToVKApiStruct(dbx.Instance, 5, currentUserID, mPeerID))
	}

	response := gin.H{
		"count": len(items),
		"items": items,
	}

	if c.Query("extended") == "1" {
		var userIDs []int64
		var groupIDs []int64

		core.CollectAllEntityIDs(items, &userIDs, &groupIDs)

		response["profiles"] = userIDs
		response["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}

func GetByConversationMessageID(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerIDStr := c.Query("peer_id")
	cmidsStr := c.Query("conversation_message_ids")

	if (peerIDStr == "" && c.Query("chat_id") == "") || cmidsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or conversation_message_ids")
		return
	}

	peerID, _ := strconv.ParseInt(peerIDStr, 10, 64)
	uIDParam, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	chatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)
	if chatID == "" && peerID != 0 {
		chatID = chat.GetInternalChatID(peerID, currentUserID)
	}

	if currentUserID != 0 {
		inChat, _ := chat.IsUserInChat(dbx.Instance, chatID, currentUserID)
		if !inChat {
			r.Reject(c, 917, "Access denied")
			return
		}
	}


	idStrings := strings.Split(cmidsStr, ",")
	var localIDs []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			localIDs = append(localIDs, id)
		}
	}

	var dbMessages []db_models.Message
	cmQuery := dbx.Instance.Where("chat_id = ? AND local_id IN ?", chatID, localIDs)
	cmQuery = db_models.BuildVisibilityFilter(cmQuery, chatID, currentUserID)
	err := cmQuery.Find(&dbMessages).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	items := make([]db_models.VKApiMessage, 0)
	for _, m := range dbMessages {
		items = append(items, m.ToVKApiStruct(dbx.Instance, 5, currentUserID, peerID))
	}

	response := gin.H{
		"count": len(items),
		"items": items,
	}

	if c.Query("extended") == "1" {
		var userIDs []int64
		var groupIDs []int64

		core.CollectAllEntityIDs(items, &userIDs, &groupIDs)

		response["profiles"] = userIDs
		response["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}
