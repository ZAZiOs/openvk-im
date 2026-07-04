package history

import (
	"net/http"
	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetHistory(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	} else if uID := c.Query("user_id"); uID != "" {
		id, _ := strconv.ParseInt(uID, 10, 64)
		peerID = id
	} else if chatID := c.Query("chat_id"); chatID != "" {
		id, _ := strconv.ParseInt(chatID, 10, 64)
		peerID = 2000000000 + id
	}

	chatID := chat.GetInternalChatID(peerID, currentUserID)

	if peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id, user_id or chat_id")
		return
	}

	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	startID, _ := strconv.ParseInt(c.Query("start_message_id"), 10, 64)
	rev, _ := strconv.Atoi(c.DefaultQuery("rev", "0"))

	if peerID > 2000000000 {
		inChat, err := chat.IsUserInChat(nil, chatID, currentUserID)
		if err != nil || !inChat {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	var msgs []db_models.Message
	query := db.Instance.Where("chat_id = ? AND deleted_at IS NULL", chatID)

	if startID > 0 {
		if offset < 0 {
			absOffset := int(-offset)

			if rev == 1 {
				query = query.Where("local_id >= (SELECT local_id FROM messages WHERE chat_id = ? AND deleted_at IS NULL AND local_id <= ? ORDER BY local_id DESC LIMIT 1 OFFSET ?)",
					chatID, startID, absOffset-1)
			} else {
				query = query.Where("local_id <= (SELECT local_id FROM messages WHERE chat_id = ? AND deleted_at IS NULL AND local_id >= ? ORDER BY local_id ASC LIMIT 1 OFFSET ?)",
					chatID, startID, absOffset-1)
			}

			offset = 0
		} else {
			if rev == 1 {
				query = query.Where("local_id >= ?", startID)
			} else {
				query = query.Where("local_id <= ?", startID)
			}
		}
	}

	order := "local_id DESC"
	if rev == 1 {
		order = "local_id ASC"
	}

	err := query.Order(order).Limit(count).Offset(offset).Find(&msgs).Error
	if err != nil {
		r.Reject(c, 10, "Internal server error during DB query")
		return
	}

	var totalCount int64
	db.Instance.Model(&db_models.Message{}).Where("chat_id = ? AND deleted_at IS NULL", chatID).Count(&totalCount)

	responseItems := make([]db_models.VKApiMessage, len(msgs))
	for i, m := range msgs {
		responseItems[i] = m.ToVKApiStruct(db.Instance, 1, currentUserID, peerID)
	}

	member, _ := chat.GetMember(db.Instance, chatID, currentUserID)
	var unreadCount int64
	if member != nil {
		db.Instance.Model(&db_models.Message{}).
			Where("chat_id = ? AND local_id > ? AND from_id != ? AND deleted_at IS NULL", chatID, member.LastReadID, currentUserID).
			Count(&unreadCount)
	}

	response := gin.H{
		"count":  totalCount,
		"items":  responseItems,
		"unread": unreadCount,
	}

	if c.Query("extended") == "1" {
		var userIDs, groupIDs, chatIDs []int64

		core.CollectAllEntityIDs(responseItems, &userIDs, &groupIDs)

		if peerID > 2000000000 {
			var members []db_models.ConversationMember
			db.Instance.Where("peer_id = ? AND left_at IS NULL", peerID).Find(&members)

			for _, m := range members {
				addID(m.UserID, &userIDs, &groupIDs, &chatIDs)
				addID(m.InvitedBy, &userIDs, &groupIDs, &chatIDs)
			}
		} else {
			participants := []int64{currentUserID, peerID}
			for _, p := range participants {
				addID(p, &userIDs, &groupIDs, &chatIDs)
			}
		}

		response["profiles"] = uniqueIDs(userIDs)
		response["groups"] = uniqueIDs(groupIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": response})
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

func addID(id int64, u *[]int64, g *[]int64, c *[]int64) {
	if id > 2000000000 {
		*c = append(*c, id)
	} else if id > 0 && id < 2000000000 {
		*u = append(*u, id)
	} else if id < 0 {
		*g = append(*g, -id)
	}
}
