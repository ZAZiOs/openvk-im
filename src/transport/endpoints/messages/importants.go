package messages

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func MarkAsImportant(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	idsStr := c.DefaultQuery("message_ids", c.PostForm("message_ids"))
	cmidsStr := c.DefaultQuery("conversation_message_ids", c.DefaultQuery("cmids", c.PostForm("conversation_message_ids")))
	peerID, _ := strconv.ParseInt(c.DefaultQuery("peer_id", c.PostForm("peer_id")), 10, 64)
	if peerID == 0 {
		if uID, err := strconv.ParseInt(c.DefaultQuery("user_id", c.PostForm("user_id")), 10, 64); err == nil && uID != 0 {
			peerID = uID
		} else if cID, err := strconv.ParseInt(c.DefaultQuery("chat_id", c.PostForm("chat_id")), 10, 64); err == nil && cID != 0 {
			if cID > 2000000000 {
				peerID = cID
			} else {
				peerID = 2000000000 + cID
			}
		}
	}

	important, _ := strconv.Atoi(c.DefaultQuery("important", c.PostForm("important")))
	if c.Query("important") == "" && c.PostForm("important") == "" {
		important = 1
	}

	var messages []db_models.Message

	if idsStr != "" {
		parts := strings.Split(idsStr, ",")
		var ids []uint64
		for _, p := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			if peerID != 0 {
				internalChatID := chat.GetInternalChatID(peerID, currentUserID)
				q := db.Instance.Where("chat_id = ? AND (id IN ? OR local_id IN ?)", internalChatID, ids, ids)
				q = db_models.BuildVisibilityFilter(q, internalChatID, currentUserID)
				q.Find(&messages)
			} else {
				q := db.Instance.Where("id IN ? AND chat_id IN (SELECT internal_chat_id FROM conversation_members WHERE user_id = ? AND left_at IS NULL)", ids, currentUserID)
				q = db_models.BuildVisibilityFilter(q, "", currentUserID)
				q.Find(&messages)
			}
		}
	} else if cmidsStr != "" && peerID != 0 {
		parts := strings.Split(cmidsStr, ",")
		var cmids []uint64
		for _, p := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
				cmids = append(cmids, id)
			}
		}
		if len(cmids) > 0 {
			internalChatID := chat.GetInternalChatID(peerID, currentUserID)
			q := db.Instance.Where("chat_id = ? AND local_id IN ?", internalChatID, cmids)
			q = db_models.BuildVisibilityFilter(q, internalChatID, currentUserID)
			q.Find(&messages)
		}
	}

	if len(messages) == 0 {
		if idsStr == "" && cmidsStr == "" {
			r.Reject(c, 100, "One of the parameters is missing: message_ids or conversation_message_ids")
			return
		}
		c.JSON(http.StatusOK, gin.H{"response": []uint64{}})
		return
	}

	successIDs := make([]uint64, 0, len(messages))

	for _, m := range messages {
		mPeerID := chat.DerivePeerID(m.ChatID, currentUserID)

		if important == 1 {
			db.Instance.Clauses(clause.OnConflict{DoNothing: true}).Create(&db_models.ImportantMessage{
				UserID:    currentUserID,
				MessageID: m.ID,
				CreatedAt: time.Now(),
			})

			r.LPRepo.PushEvent(c.Request.Context(), currentUserID, "msg_set_flags", lp_models.MsgSetFlagsEvent{
				MessageID: m.ID,
				Mask:      lp_models.MessageFlags{Value: lp_models.FlagImportant},
				PeerID:    mPeerID,
			})
			successIDs = append(successIDs, m.ID)
		} else {
			db.Instance.Where("user_id = ? AND message_id = ?", currentUserID, m.ID).Delete(&db_models.ImportantMessage{})

			r.LPRepo.PushEvent(c.Request.Context(), currentUserID, "msg_reset_flags", lp_models.MsgResetFlagsEvent{
				MessageID: m.ID,
				Mask:      lp_models.MessageFlags{Value: lp_models.FlagImportant},
				PeerID:    mPeerID,
			})
			successIDs = append(successIDs, m.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"response": successIDs})
}

func getPeerType(internalChatId string) string {
	if strings.HasPrefix(internalChatId, "c") {
		return "chat"
	}
	if strings.HasPrefix(internalChatId, "g") {
		return "group"
	}
	return "user"
}

func GetImportantMessages(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)
	apiV := core.GetApiV(c)

	count, _ := strconv.Atoi(c.DefaultQuery("count", "20"))
	if count > 200 {
		count = 200
	} else if count < 1 {
		count = 20
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	previewLen, _ := strconv.Atoi(c.DefaultQuery("preview_length", "0"))
	startID, _ := strconv.ParseUint(c.DefaultQuery("start_message_id", "0"), 10, 64)

	var msgs []db_models.Message

	baseQuery := db.Instance.Model(&db_models.Message{}).
		Joins("JOIN important_messages ON important_messages.message_id = messages.id AND important_messages.user_id = ?", currentUserID)

	baseQuery = db_models.BuildVisibilityFilter(baseQuery, "", currentUserID)

	if startID > 0 {
		baseQuery = baseQuery.Where("messages.id < ?", startID)
	}

	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	err := baseQuery.Select("messages.*").
		Preload("Conversation").
		Order("important_messages.created_at DESC, messages.id DESC").
		Limit(count).Offset(offset).
		Find(&msgs).Error

	if err != nil {
		r.Reject(c, 10, "Internal server error")
		return
	}

	var extraMsgIDs []uint64
	var chatIDsToFetchMembers []string
	var targetChatIDs []string
	chatIDsMap := make(map[string]bool)

	for _, m := range msgs {
		if !chatIDsMap[m.ChatID] {
			chatIDsMap[m.ChatID] = true
			targetChatIDs = append(targetChatIDs, m.ChatID)
		}
		if strings.HasPrefix(m.ChatID, "c") {
			chatIDsToFetchMembers = append(chatIDsToFetchMembers, m.ChatID)
		}
		if m.ForwardMessages != "" {
			ids := strings.Split(m.ForwardMessages, ",")
			for _, idStr := range ids {
				if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64); err == nil {
					extraMsgIDs = append(extraMsgIDs, id)
				}
			}
		}
		if m.ReplyTo != nil && *m.ReplyTo > 0 {
			extraMsgIDs = append(extraMsgIDs, *m.ReplyTo)
		}
	}

	preloadedMap := make(map[uint64]db_models.Message)
	if len(extraMsgIDs) > 0 && len(targetChatIDs) > 0 {
		var extras []db_models.Message
		db.Instance.Where("chat_id IN ? AND local_id IN ?", targetChatIDs, extraMsgIDs).Find(&extras)
		for _, e := range extras {
			preloadedMap[e.LocalID] = e
		}
	}

	readCache := make(map[string][]db_models.MemberReadState)
	if len(targetChatIDs) > 0 {
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

	chatMembersMap := make(map[string][]int64)
	adminMap := make(map[string]int64)
	if len(chatIDsToFetchMembers) > 0 {
		type ChatMember struct {
			InternalChatID string
			UserID         int64
			IsAdmin        bool
		}
		var members []ChatMember
		db.Instance.Table("conversation_members").
			Select("internal_chat_id, user_id, is_admin").
			Where("internal_chat_id IN ? AND left_at IS NULL", chatIDsToFetchMembers).
			Order("joined_at ASC").
			Find(&members)

		for _, mem := range members {
			chatMembersMap[mem.InternalChatID] = append(chatMembersMap[mem.InternalChatID], mem.UserID)
			if mem.IsAdmin && adminMap[mem.InternalChatID] == 0 {
				adminMap[mem.InternalChatID] = mem.UserID
			}
		}
	}

	var userIDs, groupIDs, chatIDs []int64

	if apiV.IsOlderThan(5, 80) {
		legacyItems := make([]db_models.VKApiMessageLegacy, 0, len(msgs))
		for _, m := range msgs {
			msgPeerID := chat.DerivePeerID(m.ChatID, currentUserID)
			vkMsg := m.ToVKApiStructBatchLegacy(db.Instance, 0, currentUserID, msgPeerID, preloadedMap, readCache, nil)
			vkMsg.Important = true
			if previewLen > 0 {
				vkMsg.Body = core.TruncateWords(vkMsg.Body, previewLen)
			}
			if strings.HasPrefix(m.ChatID, "c") {
				if mems, ok := chatMembersMap[m.ChatID]; ok {
					vkMsg.UsersCount = len(mems)
					activeCount := 10
					if len(mems) < activeCount {
						activeCount = len(mems)
					}
					vkMsg.ChatActive = mems[:activeCount]
				}
				if adm, ok := adminMap[m.ChatID]; ok && vkMsg.AdminID == 0 {
					vkMsg.AdminID = adm
				}
			}
			legacyItems = append(legacyItems, vkMsg)
		}

		result := gin.H{
			"count": totalCount,
			"items": legacyItems,
		}

		if c.Query("extended") == "1" {
			core.CollectAllEntityIDsLegacy(legacyItems, &userIDs, &groupIDs, &chatIDs)
			for _, mems := range chatMembersMap {
				for _, memID := range mems {
					if memID > 0 {
						userIDs = append(userIDs, memID)
					}
				}
			}
			result["profiles"] = core.UniqueIDs(userIDs)
			result["groups"] = core.UniqueIDs(groupIDs)
			if len(chatIDs) > 0 {
				result["chats"] = core.UniqueIDs(chatIDs)
			}
		}

		c.JSON(http.StatusOK, gin.H{"response": result})
	} else {
		modernItems := make([]db_models.VKApiMessage, 0, len(msgs))
		for _, m := range msgs {
			msgPeerID := chat.DerivePeerID(m.ChatID, currentUserID)
			vkMsg := m.ToVKApiStructBatch(db.Instance, 0, currentUserID, msgPeerID, preloadedMap, readCache, nil)
			vkMsg.Important = true
			if previewLen > 0 {
				vkMsg.Text = core.TruncateWords(vkMsg.Text, previewLen)
			}
			modernItems = append(modernItems, vkMsg)
		}

		core.CollectAllEntityIDs(modernItems, &userIDs, &groupIDs)

		var convMembers []db_models.ConversationMember
		if len(targetChatIDs) > 0 {
			db.Instance.Where("user_id = ? AND internal_chat_id IN ? AND left_at IS NULL", currentUserID, targetChatIDs).Find(&convMembers)
		}
		conversationsList := make([]gin.H, 0, len(convMembers))
		for _, cm := range convMembers {
			pID := chat.DerivePeerID(cm.InternalChatID, currentUserID)
			conversationsList = append(conversationsList, gin.H{
				"peer": gin.H{
					"id":   pID,
					"type": getPeerType(cm.InternalChatID),
				},
				"last_message_id": cm.LastMessageID,
				"in_read":         cm.LastReadID,
				"out_read":        cm.LastMessageID,
			})
		}

		result := gin.H{
			"messages": gin.H{
				"count": totalCount,
				"items": modernItems,
			},
			"profiles":      core.UniqueIDs(userIDs),
			"groups":        core.UniqueIDs(groupIDs),
			"conversations": conversationsList,
		}

		c.JSON(http.StatusOK, gin.H{"response": result})
	}
}
