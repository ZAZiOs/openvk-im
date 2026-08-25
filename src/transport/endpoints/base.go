package endpoints

import (
	"strings"

	"github.com/gin-gonic/gin"

	"ovk-im/src/transport/endpoints/chats"
	"ovk-im/src/transport/endpoints/core"
	"ovk-im/src/transport/endpoints/custom"
	"ovk-im/src/transport/endpoints/history"
	lp_ep "ovk-im/src/transport/endpoints/longpoll"
	"ovk-im/src/transport/endpoints/messages"
	"ovk-im/src/transport/endpoints/status"
)

type Router struct {
	core.BaseHandler
}

func (r *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			key = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		}

		if key == "" {
			r.Reject(c, 5, "User authorization failed: no key passed")
			c.Abort()
			return
		}

		userID, err := r.LPRepo.GetUserIDBySession(c.Request.Context(), key)
		if err != nil {
			r.Reject(c, 5, "User authorization failed: invalid session")
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func (r *Router) Register(group *gin.RouterGroup) {
	protected := group.Group("/")
	protected.Use(r.AuthMiddleware())

	protected.Match([]string{"GET", "POST"}, "/:slug", r.BasicHandler)
}

func (r *Router) BasicHandler(c *gin.Context) {
	if c.IsAborted() {
		return
	}

	slug := c.Param("slug")

	methods := map[string]func(*gin.Context, *core.BaseHandler){
		"messages.send":                       messages.Send,
		"messages.edit":                       messages.Edit,
		"messages.search":                     messages.Search,
		"messages.pin":                        messages.Pin,
		"messages.unpin":                      messages.Unpin,
		"messages.markAsImportant":            messages.MarkAsImportant,
		"messages.getImportantMessages":       messages.GetImportantMessages,
		"messages.markAsRead":                 messages.MarkAsRead,
		"messages.delete":                     messages.Delete,
		"messages.restore":                    messages.Restore,
		"messages.getById":                    messages.GetByID,
		"messages.getByConversationMessageId": messages.GetByConversationMessageID,

		"messages.getHistory":            history.GetHistory,
		"messages.getHistoryAttachments": history.GetHistoryAttachments,

		"messages.createChat":                  chats.CreateChat,
		"messages.editChat":                    chats.EditChat,
		"messages.setChatPhoto":                chats.SetChatPhoto,
		"messages.deleteChatPhoto":             chats.DeleteChatPhoto,
		"messages.addChatUser":                 chats.AddChatUser,
		"messages.removeChatUser":              chats.RemoveChatUser,
		"messages.getConversations":            chats.GetConversations,
		"messages.getConversationMembers":      chats.GetConversationMembers,
		"messages.getConversationsById":        chats.GetConversationsById,
		"messages.searchConversations":         chats.SearchConversations,
		"messages.markAsAnsweredConversation":  chats.MarkAsAnsweredConversation,
		"messages.markAsImportantConversation": chats.MarkAsImportantConversation,
		"messages.deleteConversation":          chats.DeleteConversation,

		"messages.setActivity": status.SetActivity,

		"messages.getLongPollServer":  lp_ep.GetLongPollServer,
		"messages.getLongPollHistory": lp_ep.GetLongPollHistory,

		"im.getUnreadMessages":      custom.GetUnreadMessages,
		"im.getUnreadConversations": custom.GetUnreadConversations,
		"im.getMe":                  custom.GetMe,
	}

	handler, exists := methods[slug]
	if !exists {
		r.Reject(c, 8, "Unknown method passed")
		return
	}

	handler(c, &r.BaseHandler)
}
