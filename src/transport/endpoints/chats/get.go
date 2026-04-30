package chats

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetConversations(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}

	filter := c.DefaultQuery("filter", "all")
	extended := c.Query("extended") == "1"

	type ResultRow struct {
		db_models.ConversationMember
		TotalCount  int64 `gorm:"column:total_count"`
		TotalUnread int64 `gorm:"column:total_unread"`
	}

	var rows []ResultRow

	query := db.Instance.Table("conversation_members").
		Select(`
			conversation_members.*, 
			COUNT(*) OVER() as total_count,
			SUM(CASE WHEN last_message_id > last_read_id THEN 1 ELSE 0 END) OVER() as total_unread
		`).
		Where("user_id = ? AND left_at IS NULL", currentUserID)

	if filter == "unread" {
		query = query.Where("last_message_id > last_read_id")
	}

	err := query.Order("last_message_id DESC").
		Preload("Conversation").
		Limit(count).Offset(offset).Find(&rows).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusOK, gin.H{"response": gin.H{"count": 0, "items": []interface{}{}, "unread_count": 0}})
		return
	}

	totalCount := rows[0].TotalCount
	totalUnreadConversations := rows[0].TotalUnread

	lastMsgKeys := make(map[string]uint64)
	unreadCheckIDs := make([]string, 0)

	for _, row := range rows {
		if row.LastMessageID > 0 {
			lastMsgKeys[row.InternalChatID] = row.LastMessageID
		}
		if row.LastMessageID > row.LastReadID {
			unreadCheckIDs = append(unreadCheckIDs, row.InternalChatID)
		}
	}

	msgMap := make(map[string]db_models.Message)
	if len(lastMsgKeys) > 0 {
		var lastMessages []db_models.Message
		db.Instance.Where("(chat_id, local_id) IN ?", buildInPairs(lastMsgKeys)).Find(&lastMessages)
		for _, msg := range lastMessages {
			msgMap[msg.ChatID] = msg
		}
	}

	unreadCounts := make(map[string]int64)
	if len(unreadCheckIDs) > 0 {
		type UnreadRes struct {
			ChatID string
			Cnt    int64
		}
		var results []UnreadRes
		db.Instance.Model(&db_models.Message{}).
			Select("chat_id, COUNT(*) as cnt").
			Where("chat_id IN ? AND from_id != ?", unreadCheckIDs, currentUserID).
			Group("chat_id").Find(&results)

		for _, res := range results {
			unreadCounts[res.ChatID] = res.Cnt
		}
	}

	preloadedMap := make(map[uint64]db_models.Message)
	extraMsgIDs := make([]uint64, 0)
	chatIDs := make([]string, 0)

	for chatID, msg := range msgMap {
		chatIDs = append(chatIDs, chatID)
		if msg.ReplyTo != nil && *msg.ReplyTo > 0 {
			extraMsgIDs = append(extraMsgIDs, *msg.ReplyTo)
		}
		if msg.ForwardMessages != "" {
			for _, idStr := range strings.Split(msg.ForwardMessages, ",") {
				if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64); err == nil {
					extraMsgIDs = append(extraMsgIDs, id)
				}
			}
		}
	}

	if len(extraMsgIDs) > 0 {
		var extras []db_models.Message
		db.Instance.Where("chat_id IN ? AND local_id IN ?", chatIDs, extraMsgIDs).Find(&extras)
		for _, e := range extras {
			preloadedMap[e.LocalID] = e
		}
	}

	responseItems := make([]gin.H, 0)
	var userIDs, groupIDs []int64

	for _, row := range rows {
		m := row.ConversationMember
		conv := m.Conversation // Из Preload
		pID := chat.DerivePeerID(m.InternalChatID, currentUserID)

		lastMsg, hasMsg := msgMap[m.InternalChatID]
		var msgVK interface{} = nil
		if hasMsg {
			// ВАЖНО: убедись, что ToVKApiStruct не делает запросов к БД внутри
			msgVK = lastMsg.ToVKApiStructBatch(1, currentUserID, pID, preloadedMap)
		}

		conversationObj := gin.H{
			"peer":            gin.H{"id": pID, "type": getPeerType(pID)},
			"last_message_id": m.LastMessageID,
			"in_read":         m.LastReadID,
			"out_read":        m.LastMessageID,
		}

		if uCount, ok := unreadCounts[m.InternalChatID]; ok {
			conversationObj["unread_count"] = uCount
		}

		if conv.PinnedMsgID > 0 {
			conversationObj["current_pinned_message"] = gin.H{"id": conv.PinnedMsgID}
		}

		responseItems = append(responseItems, gin.H{
			"conversation": conversationObj,
			"last_message": msgVK,
		})

		if extended {
			if hasMsg {
				addID(lastMsg.FromID, &userIDs, &groupIDs)
			}
			addID(pID, &userIDs, &groupIDs)
		}
	}

	result := gin.H{
		"count":        totalCount,
		"items":        responseItems,
		"unread_count": totalUnreadConversations,
	}

	if extended {
		result["profiles"] = uniqueIDs(userIDs)
		result["groups"] = uniqueIDs(groupIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}

func getPeerType(peerID int64) string {
	if peerID > 2000000000 {
		return "chat"
	}
	if peerID < 0 {
		return "community"
	}
	return "user"
}

func uniqueIDs(ids []int64) []int64 {
	m := make(map[int64]bool)
	var res []int64
	for _, id := range ids {
		if !m[id] {
			m[id] = true
			res = append(res, id)
		}
	}
	return res
}

func buildInPairs(keys map[string]uint64) [][]interface{} {
	res := make([][]interface{}, 0, len(keys))
	for k, v := range keys {
		res = append(res, []interface{}{k, v})
	}
	return res
}

func addID(id int64, u *[]int64, g *[]int64) {
	if id > 0 && id < 2000000000 {
		*u = append(*u, id)
	} else if id < 0 {
		*g = append(*g, -id)
	}
}
