package messages

import (
	"net/http"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/transport/endpoints/core"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

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
	err := dbx.Instance.Where("id IN ? AND chat_id IN (SELECT internal_chat_id FROM conversation_members WHERE user_id = ? AND left_at IS NULL)",
		ids, currentUserID).Find(&dbMessages).Error

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

		collectAllEntityIDs(items, &userIDs, &groupIDs)

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

	if peerIDStr == "" || cmidsStr == "" {
		r.Reject(c, 100, "One of the parameters is missing: peer_id or conversation_message_ids")
		return
	}

	peerID, _ := strconv.ParseInt(peerIDStr, 10, 64)
	chatID := chat.GetInternalChatID(peerID, currentUserID)

	inChat, _ := chat.IsUserInChat(dbx.Instance, chatID, currentUserID)
	if !inChat {
		r.Reject(c, 917, "Access denied")
		return
	}

	idStrings := strings.Split(cmidsStr, ",")
	var localIDs []uint64
	for _, s := range idStrings {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
			localIDs = append(localIDs, id)
		}
	}

	var dbMessages []db_models.Message
	err := dbx.Instance.Where("chat_id = ? AND local_id IN ?", chatID, localIDs).Find(&dbMessages).Error

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

		collectAllEntityIDs(items, &userIDs, &groupIDs)

		response["profiles"] = userIDs
		response["groups"] = groupIDs
	}

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}

func collectAllEntityIDs(items []db_models.VKApiMessage, userIDs *[]int64, groupIDs *[]int64) {
	uMap := make(map[int64]struct{})
	gMap := make(map[int64]struct{})

	var scan func(m db_models.VKApiMessage)
	scan = func(m db_models.VKApiMessage) {
		if m.FromID > 0 {
			uMap[m.FromID] = struct{}{}
		} else if m.FromID < 0 {
			gMap[-m.FromID] = struct{}{}
		}

		if m.ReplyMessage != nil {
			scan(*m.ReplyMessage)
		}

		for _, fwd := range m.ForwardMessages {
			scan(fwd)
		}
	}

	for _, itm := range items {
		scan(itm)
	}

	for id := range uMap {
		*userIDs = append(*userIDs, id)
	}
	for id := range gMap {
		*groupIDs = append(*groupIDs, id)
	}
}
