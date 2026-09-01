package chats

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
	"gorm.io/gorm"
)

func generateInviteCode() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func extractInviteCode(link string) string {
	link = strings.TrimSpace(link)
	if idx := strings.Index(link, "join="); idx != -1 {
		link = link[idx+len("join="):]
	} else if idx := strings.Index(link, "invite="); idx != -1 {
		link = link[idx+len("invite="):]
	} else if idx := strings.LastIndex(link, "/join/"); idx != -1 {
		link = link[idx+len("/join/"):]
	} else if idx := strings.LastIndex(link, "/"); idx != -1 {
		link = link[idx+1:]
	}
	if qIdx := strings.Index(link, "&"); qIdx != -1 {
		link = link[:qIdx]
	}
	if qIdx := strings.Index(link, "?"); qIdx != -1 {
		link = link[:qIdx]
	}
	if qIdx := strings.Index(link, "#"); qIdx != -1 {
		link = link[:qIdx]
	}
	return strings.TrimSpace(link)
}

func GetInviteLink(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	peerID, _ := strconv.ParseInt(c.Query("peer_id"), 10, 64)
	if peerID == 0 {
		if cID, err := strconv.ParseInt(c.Query("chat_id"), 10, 64); err == nil && cID > 0 {
			if cID > 2000000000 {
				peerID = cID
			} else {
				peerID = 2000000000 + cID
			}
		}
	}

	if peerID <= 2000000000 {
		r.Reject(c, 100, "One of the parameters is missing or invalid: peer_id must be a group chat (> 2000000000)")
		return
	}

	internalChatID := fmt.Sprintf("c%d", peerID-2000000000)

	member, err := chat.GetMember(db.Instance, internalChatID, currentUserID)
	if err != nil || member == nil || member.LeftAt != nil {
		r.Reject(c, 917, "You don't have access to this chat")
		return
	}

	if !member.IsAdmin {
		r.Reject(c, 925, "You are not admin of this chat")
		return
	}

	reset := c.Query("reset") == "1"
	if reset {
		db.Instance.Model(&db_models.ChatInvite{}).
			Where("internal_chat_id = ? AND revoked = ?", internalChatID, false).
			Update("revoked", true)
	}

	var activeInvite db_models.ChatInvite
	q := db.Instance.Where("internal_chat_id = ? AND revoked = ?", internalChatID, false)
	q = q.Where("expires_at IS NULL OR expires_at > ?", time.Now())
	q = q.Where("usage_limit = 0 OR usage_count < usage_limit")
	err = q.Order("created_at DESC").First(&activeInvite).Error

	if err != nil {
		code := generateInviteCode()
		activeInvite = db_models.ChatInvite{
			Code:           code,
			InternalChatID: internalChatID,
			CreatorID:      currentUserID,
			Revoked:        false,
			CreatedAt:      time.Now(),
		}
		if err := db.Instance.Create(&activeInvite).Error; err != nil {
			r.Reject(c, 10, "Internal server error: "+err.Error())
			return
		}
	}

	link := activeInvite.Code

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"link": link,
		},
	})
}

func GetChatPreview(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	rawLink := c.Query("link")
	if rawLink == "" {
		r.Reject(c, 100, "One of the parameters is missing: link")
		return
	}

	code := extractInviteCode(rawLink)
	if code == "" {
		r.Reject(c, 947, "Chat invite link is invalid")
		return
	}

	var invite db_models.ChatInvite
	err := db.Instance.Where("code = ? AND revoked = ?", code, false).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		First(&invite).Error

	if err != nil {
		r.Reject(c, 947, "Chat invite link is invalid or expired")
		return
	}

	var conv db_models.Conversation
	if err := db.Instance.Where("internal_id = ?", invite.InternalChatID).First(&conv).Error; err != nil {
		r.Reject(c, 947, "Chat not found")
		return
	}

	var membersCount int64
	db.Instance.Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND left_at IS NULL", invite.InternalChatID).
		Count(&membersCount)

	var adminMember db_models.ConversationMember
	db.Instance.Where("internal_chat_id = ? AND is_admin = ? AND left_at IS NULL", invite.InternalChatID, true).
		Order("joined_at ASC").
		First(&adminMember)

	isMember := false
	if currentUserID > 0 {
		var userMember db_models.ConversationMember
		if err := db.Instance.Where("internal_chat_id = ? AND user_id = ? AND left_at IS NULL", invite.InternalChatID, currentUserID).First(&userMember).Error; err == nil {
			isMember = true
		}
	}

	localChatID, _ := strconv.ParseInt(strings.TrimPrefix(invite.InternalChatID, "c"), 10, 64)

	var memberIDs []int64
	db.Instance.Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND left_at IS NULL", invite.InternalChatID).
		Limit(10).
		Pluck("user_id", &memberIDs)

	preview := gin.H{
		"admin_id":      adminMember.UserID,
		"members_count": membersCount,
		"title":         conv.Title,
		"photo": gin.H{
			"photo_50":  "",
			"photo_100": "",
			"photo_200": "",
		},
		"local_id":  localChatID,
		"is_member": isMember,
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"preview":  preview,
			"profiles": core.UniqueIDs(memberIDs),
			"emails":   []string{},
		},
	})
}

func JoinChatByInviteLink(c *gin.Context, r *core.BaseHandler) {
	val, _ := c.Get("userID")
	currentUserID := val.(int64)

	rawLink := c.Query("link")
	if rawLink == "" {
		r.Reject(c, 100, "One of the parameters is missing: link")
		return
	}

	code := extractInviteCode(rawLink)
	if code == "" {
		r.Reject(c, 947, "Chat invite link is invalid")
		return
	}

	var invite db_models.ChatInvite
	err := db.Instance.Where("code = ? AND revoked = ?", code, false).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Where("usage_limit = 0 OR usage_count < usage_limit").
		First(&invite).Error

	if err != nil {
		r.Reject(c, 947, "Chat invite link is invalid or expired")
		return
	}

	localChatID, _ := strconv.ParseInt(strings.TrimPrefix(invite.InternalChatID, "c"), 10, 64)
	peerID := 2000000000 + localChatID

	var existing db_models.ConversationMember
	if err := db.Instance.Where("internal_chat_id = ? AND user_id = ? AND left_at IS NULL", invite.InternalChatID, currentUserID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"response": gin.H{
				"chat_id": localChatID,
			},
		})
		return
	}

	messageText := "joined by invite link"
	msg, err := chat.AddUserToConversation(
		invite.InternalChatID,
		currentUserID,
		currentUserID,
		messageText,
		"chat_invite_user_by_link",
		currentUserID,
		"",
	)

	if err != nil {
		r.Reject(c, 15, "Failed to join chat: "+err.Error())
		return
	}

	db.Instance.Model(&invite).UpdateColumn("usage_count", gorm.Expr("usage_count + 1"))

	participants, err := chat.GetActiveMemberIDs(nil, invite.InternalChatID)
	if err == nil {
		lpAttach := lp_models.LPAttachments{
			Source: "chat_invite_user_by_link",
			From:   strconv.FormatInt(currentUserID, 10),
			Emoji:  false,
		}

		baseEvent := lp_models.NewMessageEvent{
			MessageID:   uint64(msg.ID),
			PeerID:      peerID,
			Timestamp:   int(msg.CreatedAt.Unix()),
			Text:        messageText,
			Attachments: &lpAttach,
		}

		go func(parts []int64, event lp_models.NewMessageEvent) {
			ctx := context.Background()
			for _, uID := range parts {
				userEvent := event
				userEvent.Flags = lp_models.MessageFlags{Value: event.Flags.Value}

				if uID == currentUserID {
					userEvent.Flags.Add(lp_models.FlagOutbox)
				} else {
					userEvent.Flags.Add(lp_models.FlagUnread)
				}

				_, _, err := r.LPRepo.PushEvent(ctx, uID, "new_msg", userEvent)
				if err == nil {
					r.Broadcaster.Notify(uID)
				}
			}
		}(participants, baseEvent)
	}

	c.JSON(http.StatusOK, gin.H{
		"response": gin.H{
			"chat_id": localChatID,
		},
	})
}
