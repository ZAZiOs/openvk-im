package history

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

func GetHistory(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	var peerID int64
	var uIDParam int64
	if pID := c.Query("peer_id"); pID != "" {
		peerID, _ = strconv.ParseInt(pID, 10, 64)
	}
	if uID := c.Query("user_id"); uID != "" {
		uIDParam, _ = strconv.ParseInt(uID, 10, 64)
		if peerID == 0 {
			peerID = uIDParam
		}
	} else if chatIDParam := c.Query("chat_id"); chatIDParam != "" {
		if id, err := strconv.ParseInt(chatIDParam, 10, 64); err == nil && peerID == 0 {
			peerID = 2000000000 + id
		}
	}

	chatID := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)
	if chatID == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id, user_id or chat_id")
		return
	}

	if peerID == 0 {
		peerID = chat.DerivePeerID(chatID, currentUserID)
	}

	isGroupChat := strings.HasPrefix(chatID, "c")

	var member *db_models.ConversationMember
	if currentUserID != 0 {
		if isGroupChat {
			var err error
			member, err = chat.GetMember(db.Instance, chatID, currentUserID)
			if err != nil || member == nil || member.LeftAt != nil {
				r.Reject(c, 917, "You don't have access to this chat")
				return
			}
		} else {
			member, _ = chat.GetMember(db.Instance, chatID, currentUserID)
			if member == nil {
				r.Reject(c, 917, "Conversation doesn't exist")
				return
			}
		}
	}


	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	startID, _ := strconv.ParseInt(c.Query("start_message_id"), 10, 64)
	rev, _ := strconv.Atoi(c.DefaultQuery("rev", "0"))

	var msgs []db_models.Message
	query := db.Instance.Where("chat_id = ?", chatID)
	query = db_models.BuildVisibilityFilter(query, chatID, currentUserID)

	if startID > 0 {
		if offset < 0 {
			absOffset := int(-offset)

			if rev == 1 {
				subQ := db.Instance.Model(&db_models.Message{}).Where("chat_id = ? AND local_id <= ?", chatID, startID)
				subQ = db_models.BuildVisibilityFilter(subQ, chatID, currentUserID)
				query = query.Where("local_id >= (?)",
					subQ.Select("local_id").Order("local_id DESC").Limit(1).Offset(absOffset-1))
			} else {
				subQ := db.Instance.Model(&db_models.Message{}).Where("chat_id = ? AND local_id >= ?", chatID, startID)
				subQ = db_models.BuildVisibilityFilter(subQ, chatID, currentUserID)
				query = query.Where("local_id <= (?)",
					subQ.Select("local_id").Order("local_id ASC").Limit(1).Offset(absOffset-1))
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
	query.Count(&totalCount)

	readCache := make(map[string][]db_models.MemberReadState)
	var memberStates []db_models.MemberReadState
	db.Instance.Table("conversation_members").
		Select("user_id, last_read_id").
		Where("internal_chat_id = ?", chatID).
		Scan(&memberStates)
	readCache[chatID] = memberStates

	responseItems := make([]db_models.VKApiMessage, len(msgs))
	for i, m := range msgs {
		responseItems[i] = m.ToVKApiStructBatch(db.Instance, 1, currentUserID, peerID, nil, readCache)
	}

	var unreadCount int64
	if member != nil {
		unreadQuery := db.Instance.Model(&db_models.Message{}).
			Where("chat_id = ? AND local_id > ? AND from_id != ?", chatID, member.LastReadID, currentUserID)
		unreadQuery = db_models.BuildVisibilityFilter(unreadQuery, chatID, currentUserID)
		unreadQuery.Count(&unreadCount)
	}

	response := gin.H{
		"count":  totalCount,
		"items":  responseItems,
		"unread": unreadCount,
	}

	if c.Query("extended") == "1" {
		var userIDs, groupIDs, chatIDs []int64

		core.CollectAllEntityIDs(responseItems, &userIDs, &groupIDs)

		if isGroupChat {
			var members []db_models.ConversationMember
			db.Instance.Where("internal_chat_id = ? AND left_at IS NULL", chatID).Find(&members)

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
