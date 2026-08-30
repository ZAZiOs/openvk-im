package chats

import (
	"fmt"
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
            SUM(CASE WHEN conversation_members.last_message_id > conversation_members.last_read_id THEN 1 ELSE 0 END) OVER() as total_unread
        `).
		Joins("LEFT JOIN messages ON messages.chat_id = conversation_members.internal_chat_id AND messages.local_id = conversation_members.last_message_id").
		Where("conversation_members.user_id = ? AND conversation_members.left_at IS NULL", currentUserID)

	if currentUserID == 0 {
		query = db.Instance.Table("conversations").
			Select(`
				conversations.internal_id as internal_chat_id,
				conversations.last_message_id as last_message_id,
				COUNT(*) OVER() as total_count,
				0 as total_unread
			`).
			Joins("LEFT JOIN messages ON messages.chat_id = conversations.internal_id AND messages.local_id = conversations.last_message_id")
	} else if filter == "unread" {
		query = query.Where("conversation_members.last_message_id > conversation_members.last_read_id")
	}


	err := query.Order("messages.created_at DESC, messages.id DESC").
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

	numRows := len(rows)
	lastMsgKeys := make(map[string]uint64, numRows)
	unreadCheckIDs := make([]string, 0, numRows)
	chatIDsToFetchMembers := make([]string, 0, numRows)

	for _, row := range rows {
		if row.LastMessageID > 0 {
			lastMsgKeys[row.InternalChatID] = row.LastMessageID
		}
		if row.LastMessageID > row.LastReadID {
			unreadCheckIDs = append(unreadCheckIDs, row.InternalChatID)
		}
		if getPeerType(row.InternalChatID) == "chat" {
			chatIDsToFetchMembers = append(chatIDsToFetchMembers, row.InternalChatID)
		}
	}

	msgMap := make(map[string]db_models.Message, len(lastMsgKeys))
	if len(lastMsgKeys) > 0 {
		var lastMessages []db_models.Message
		db.Instance.Where("(chat_id, local_id) IN ?", buildInPairs(lastMsgKeys)).Find(&lastMessages)
		for _, msg := range lastMessages {
			msgMap[msg.ChatID] = msg
		}
	}

	unreadCounts := make(map[string]int64, len(unreadCheckIDs))
	if len(unreadCheckIDs) > 0 {
		type UnreadRes struct {
			ChatID string
			Cnt    int64
		}
		var results []UnreadRes

		db.Instance.Table("messages").
			Select("messages.chat_id, COUNT(messages.id) as cnt").
			Joins("JOIN conversation_members ON conversation_members.internal_chat_id = messages.chat_id").
			Where("conversation_members.user_id = ?", currentUserID).
			Where("messages.chat_id IN ?", unreadCheckIDs).
			Where("messages.from_id != ?", currentUserID).
			Where("messages.local_id > conversation_members.last_read_id").
			Where("messages.local_id > conversation_members.deleted_before_id").
			Where("messages.deleted_at IS NULL").
			Group("messages.chat_id").Find(&results)

		for _, res := range results {
			unreadCounts[res.ChatID] = res.Cnt
		}
	}

	chatMembersMap := make(map[string][]int64, len(chatIDsToFetchMembers))
	adminMap := make(map[string]int64, len(chatIDsToFetchMembers))

	if len(chatIDsToFetchMembers) > 0 {
		type ChatMember struct {
			InternalChatID string
			UserID         int64
		}
		var members []ChatMember
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id").
			Where("internal_chat_id IN ? AND left_at IS NULL", chatIDsToFetchMembers).
			Find(&members)

		for _, m := range members {
			chatMembersMap[m.InternalChatID] = append(chatMembersMap[m.InternalChatID], m.UserID)
		}

		var admins []ChatMember
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id").
			Where("internal_chat_id IN ? AND is_admin = ? AND left_at IS NULL", chatIDsToFetchMembers, true).
			Find(&admins)

		for _, a := range admins {
			adminMap[a.InternalChatID] = a.UserID
		}
	}

	preloadedMap := make(map[uint64]db_models.Message)
	extraMsgIDs := make([]uint64, 0, len(msgMap))
	convIDs := make([]string, 0, len(msgMap))

	for chatID, msg := range msgMap {
		convIDs = append(convIDs, chatID)
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
		db.Instance.Where("chat_id IN ? AND local_id IN ?", convIDs, extraMsgIDs).Find(&extras)
		for _, e := range extras {
			preloadedMap[e.LocalID] = e
		}
	}

	readCache := make(map[string][]db_models.MemberReadState)
	if len(lastMsgKeys) > 0 {
		var targetChatIDs []string
		for chatID := range lastMsgKeys {
			targetChatIDs = append(targetChatIDs, chatID)
		}
		var memberStates []struct {
			InternalChatID string `gorm:"column:internal_chat_id"`
			UserID         int64  `gorm:"column:user_id"`
			LastReadID     uint64 `gorm:"column:last_read_id"`
		}
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id, last_read_id").
			Where("internal_chat_id IN ?", targetChatIDs).
			Scan(&memberStates)
		for _, ms := range memberStates {
			readCache[ms.InternalChatID] = append(readCache[ms.InternalChatID], db_models.MemberReadState{
				UserID:     ms.UserID,
				LastReadID: ms.LastReadID,
			})
		}
	}

	responseItems := make([]gin.H, 0)
	var userIDs, groupIDs, chatIDs []int64

	for _, row := range rows {
		m := row.ConversationMember
		conv := m.Conversation
		pID := chat.DerivePeerID(m.InternalChatID, currentUserID)
		lastMsg, hasMsg := msgMap[m.InternalChatID]

		var msgVK interface{} = nil
		if hasMsg {
			msgVK = lastMsg.ToVKApiStructBatch(db.Instance, 1, currentUserID, pID, preloadedMap, readCache, nil)
		}

		var majorID int64 = 0
		var minorID uint64 = m.LastMessageID
		if hasMsg {
			majorID = lastMsg.CreatedAt.Unix()
			minorID = lastMsg.LocalID
		}

		conversationObj := gin.H{
			"peer":            gin.H{"id": pID, "type": getPeerType(m.InternalChatID)},
			"last_message_id": m.LastMessageID,
			"in_read":         m.LastReadID,
			"out_read":        m.LastMessageID,
			"important":       (m.Flags & 1) != 0,
			"unanswered":      (m.Flags & 2) != 0,
			"sort_id": gin.H{
				"major_id": majorID,
				"minor_id": minorID,
			},
		}

		if uCount, ok := unreadCounts[m.InternalChatID]; ok {
			conversationObj["unread_count"] = uCount
		} else {
			conversationObj["unread_count"] = 0
		}

		if conv.PinnedMsgID > 0 {
			conversationObj["current_pinned_message"] = gin.H{"id": conv.PinnedMsgID}
		}

		if getPeerType(m.InternalChatID) == "chat" {
			membersList := chatMembersMap[m.InternalChatID]
			if membersList == nil {
				membersList = []int64{}
			}
			conversationObj["chat_settings"] = gin.H{
				"members":  membersList,
				"admin_id": adminMap[m.InternalChatID],
			}
		}

		responseItems = append(responseItems, gin.H{
			"conversation": conversationObj,
			"last_message": msgVK,
		})

		if extended {
			if hasMsg {
				addID(lastMsg.FromID, &userIDs, &groupIDs, &chatIDs)
			}
			addID(pID, &userIDs, &groupIDs, &chatIDs)

			if getPeerType(m.InternalChatID) == "chat" {
				if membersList, ok := chatMembersMap[m.InternalChatID]; ok {
					for _, memberID := range membersList {
						addID(memberID, &userIDs, &groupIDs, &chatIDs)
					}
				}
			}
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

		uniqueChatIDs := uniqueIDs(chatIDs)

		extendedChats := make([]gin.H, 0, len(uniqueChatIDs))
		internalIDs := make([]string, 0, len(uniqueChatIDs))

		for _, id := range uniqueChatIDs {
			if id > 2000000000 {
				internalIDs = append(internalIDs, fmt.Sprintf("c%d", id-2000000000))
			}
		}

		adminMap := make(map[string]int64, len(internalIDs))
		if len(internalIDs) > 0 {
			type ChatAdmin struct {
				InternalChatID string
				UserID         int64
			}
			var admins []ChatAdmin
			db.Instance.Table("conversation_members").
				Select("internal_chat_id, user_id").
				Where("internal_chat_id IN ? AND is_admin = ? AND left_at IS NULL", internalIDs, true).
				Find(&admins)

			for _, a := range admins {
				adminMap[a.InternalChatID] = a.UserID
			}
		}

		for _, id := range uniqueChatIDs {
			if id <= 2000000000 {
				continue
			}

			localID := id - 2000000000
			intKey := fmt.Sprintf("c%d", localID)

			members := chatMembersMap[intKey]
			if members == nil {
				members = []int64{}
			}

			extendedChats = append(extendedChats, gin.H{
				"id":          id,
				"type":        "chat",
				"admin_id":    adminMap[intKey],
				"left":        0,
				"kicked":      0,
				"title":       "",
				"description": "",
				"photo_50":    "",
				"photo_100":   "",
				"photo_200":   "",
				"members":     members,
			})
		}

		result["chats"] = extendedChats
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}

func GetConversationMembers(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	uIDParam, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	internalChatId := chat.ResolveChatID(c.Query("chat_id"), peerID, uIDParam, currentUserID)

	if internalChatId == "" && peerID == 0 {
		r.Reject(c, 100, "One of the parameters is missing: peer_id")
		return
	}
	if internalChatId == "" {
		internalChatId = chat.GetInternalChatID(peerID, currentUserID)
	}

	extended := c.Query("extended") == "1"

	if currentUserID != 0 {
		var check db_models.ConversationMember
		err := db.Instance.Where("internal_chat_id = ? AND user_id = ? AND left_at IS NULL", internalChatId, currentUserID).First(&check).Error

		if err != nil {
			r.Reject(c, 917, "You don't have access to this chat")
			return
		}
	}

	var members []db_models.ConversationMember
	var userIDs, groupIDs, chatIDs []int64
	items := make([]gin.H, 0)

	if peerID > 2000000000 || strings.HasPrefix(internalChatId, "c") {
		db.Instance.Where("internal_chat_id = ? AND left_at IS NULL", internalChatId).Find(&members)

		for _, m := range members {
			item := gin.H{
				"member_id":  m.UserID,
				"invited_by": m.InvitedBy,
				"join_date":  m.JoinedAt.Unix(),
			}
			if m.IsAdmin {
				item["is_admin"] = true
			}
			items = append(items, item)

			if extended {
				addID(m.UserID, &userIDs, &groupIDs, &chatIDs)
				addID(m.InvitedBy, &userIDs, &groupIDs, &chatIDs)
			}
		}
	} else {
		participants := []int64{currentUserID, peerID}
		for _, p := range participants {
			items = append(items, gin.H{
				"member_id": p,
			})
			if extended {
				addID(p, &userIDs, &groupIDs, &chatIDs)
			}
		}
	}

	result := gin.H{
		"count": len(items),
		"items": items,
	}

	if extended {
		result["profiles"] = uniqueIDs(userIDs)
		result["groups"] = uniqueIDs(groupIDs)
		result["chats"] = uniqueIDs(chatIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}

func GetConversationsById(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerIDsStr := c.Query("peer_ids")
	if peerIDsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_ids")
		return
	}

	extended := c.Query("extended") == "1"
	parts := strings.Split(peerIDsStr, ",")
	var targetChatIDs []string
	for _, p := range parts {
		pTrim := strings.TrimSpace(p)
		if strings.HasPrefix(pTrim, "dm") || strings.HasPrefix(pTrim, "c") || strings.HasPrefix(pTrim, "g") {
			targetChatIDs = append(targetChatIDs, pTrim)
		} else if id, err := strconv.ParseInt(pTrim, 10, 64); err == nil {
			internalID := chat.GetInternalChatID(id, currentUserID)
			targetChatIDs = append(targetChatIDs, internalID)
		}
	}

	var rows []db_models.ConversationMember
	var err error
	if currentUserID == 0 {
		var convs []db_models.Conversation
		err = db.Instance.Where("internal_id IN ?", targetChatIDs).Find(&convs).Error
		for _, conv := range convs {
			rows = append(rows, db_models.ConversationMember{
				InternalChatID: conv.InternalID,
				LastMessageID:  conv.LastMessageID,
				Conversation:   conv,
			})
		}
	} else {
		err = db.Instance.Where("user_id = ? AND internal_chat_id IN ? AND left_at IS NULL", currentUserID, targetChatIDs).
			Preload("Conversation").
			Find(&rows).Error
	}


	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	lastMsgKeys := make(map[string]uint64)
	chatIDsToFetchMembers := make([]string, 0)
	for _, row := range rows {
		if row.LastMessageID > 0 {
			lastMsgKeys[row.InternalChatID] = row.LastMessageID
		}
		if getPeerType(row.InternalChatID) == "chat" {
			chatIDsToFetchMembers = append(chatIDsToFetchMembers, row.InternalChatID)
		}
	}

	chatMembersMap := make(map[string][]int64)
	adminMap := make(map[string]int64, len(chatIDsToFetchMembers))
	if len(chatIDsToFetchMembers) > 0 {
		type ChatMember struct {
			InternalChatID string
			UserID         int64
		}
		var members []ChatMember
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id").
			Where("internal_chat_id IN ? AND left_at IS NULL", chatIDsToFetchMembers).
			Find(&members)

		for _, m := range members {
			chatMembersMap[m.InternalChatID] = append(chatMembersMap[m.InternalChatID], m.UserID)
		}

		var admins []ChatMember
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id").
			Where("internal_chat_id IN ? AND is_admin = ? AND left_at IS NULL", chatIDsToFetchMembers, true).
			Find(&admins)

		for _, a := range admins {
			adminMap[a.InternalChatID] = a.UserID
		}
	}

	msgMap := make(map[string]db_models.Message)
	extraMsgIDs := make([]uint64, 0)
	convIDs := make([]string, 0)

	if len(lastMsgKeys) > 0 {
		var lastMessages []db_models.Message
		db.Instance.Where("(chat_id, local_id) IN ?", buildInPairs(lastMsgKeys)).Find(&lastMessages)

		for _, msg := range lastMessages {
			msgMap[msg.ChatID] = msg
			convIDs = append(convIDs, msg.ChatID)

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
	}

	preloadedMap := make(map[uint64]db_models.Message)
	if len(extraMsgIDs) > 0 {
		var extras []db_models.Message
		db.Instance.Where("chat_id IN ? AND local_id IN ?", convIDs, extraMsgIDs).Find(&extras)
		for _, e := range extras {
			preloadedMap[e.LocalID] = e
		}
	}

	readCache := make(map[string][]db_models.MemberReadState)
	if len(lastMsgKeys) > 0 {
		var targetChatIDs []string
		for chatID := range lastMsgKeys {
			targetChatIDs = append(targetChatIDs, chatID)
		}
		var memberStates []struct {
			InternalChatID string `gorm:"column:internal_chat_id"`
			UserID         int64  `gorm:"column:user_id"`
			LastReadID     uint64 `gorm:"column:last_read_id"`
		}
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id, last_read_id").
			Where("internal_chat_id IN ?", targetChatIDs).
			Scan(&memberStates)
		for _, ms := range memberStates {
			readCache[ms.InternalChatID] = append(readCache[ms.InternalChatID], db_models.MemberReadState{
				UserID:     ms.UserID,
				LastReadID: ms.LastReadID,
			})
		}
	}

	responseItems := make([]gin.H, 0)
	var userIDs, groupIDs, chatIDs []int64

	for _, m := range rows {
		pID := chat.DerivePeerID(m.InternalChatID, currentUserID)
		lastMsg, hasMsg := msgMap[m.InternalChatID]

		var msgVK interface{} = nil
		if hasMsg {
			msgVK = lastMsg.ToVKApiStructBatch(db.Instance, 1, currentUserID, pID, preloadedMap, readCache, nil)
		}

		var majorID int64 = 0
		var minorID uint64 = m.LastMessageID
		if hasMsg {
			majorID = lastMsg.CreatedAt.Unix()
			minorID = lastMsg.LocalID
		}

		convObj := gin.H{
			"peer":            gin.H{"id": pID, "type": getPeerType(m.InternalChatID)},
			"last_message_id": m.LastMessageID,
			"in_read":         m.LastReadID,
			"out_read":        m.LastMessageID,
			"important":       (m.Flags & 1) != 0,
			"unanswered":      (m.Flags & 2) != 0,
			"sort_id": gin.H{
				"major_id": majorID,
				"minor_id": minorID,
			},
		}

		if m.Conversation.ID != 0 && m.Conversation.PinnedMsgID > 0 {
			convObj["current_pinned_message"] = gin.H{"id": m.Conversation.PinnedMsgID}
		}

		if getPeerType(m.InternalChatID) == "chat" {
			membersList := chatMembersMap[m.InternalChatID]
			if membersList == nil {
				membersList = []int64{}
			}
			convObj["chat_settings"] = gin.H{
				"members":  membersList,
				"admin_id": adminMap[m.InternalChatID],
			}
		}

		responseItems = append(responseItems, gin.H{
			"conversation": convObj,
			"last_message": msgVK,
		})

		if extended {
			addID(pID, &userIDs, &groupIDs, &chatIDs)
			if hasMsg {
				addID(lastMsg.FromID, &userIDs, &groupIDs, &chatIDs)
			}

			if getPeerType(m.InternalChatID) == "chat" {
				if membersList, ok := chatMembersMap[m.InternalChatID]; ok {
					for _, memberID := range membersList {
						addID(memberID, &userIDs, &groupIDs, &chatIDs)
					}
				}
			}
		}
	}

	result := gin.H{
		"count": len(responseItems),
		"items": responseItems,
	}

	if extended {
		result["profiles"] = uniqueIDs(userIDs)
		result["groups"] = uniqueIDs(groupIDs)
		result["chats"] = uniqueIDs(chatIDs)
	}

	c.JSON(http.StatusOK, gin.H{"response": result})
}

func getPeerType(internalChatId string) string {
	if strings.HasPrefix(internalChatId, "c") {
		return "chat"
	}
	if strings.HasPrefix(internalChatId, "g") {
		return "community"
	}
	if strings.HasPrefix(internalChatId, "dm") {
		return "user"
	}
	return "unknown"
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
