package chat

import (
	"errors"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrGroupNoPermission = errors.New("community can not start conversation")
	ErrChatNotFound      = errors.New("conversation not found")
)

func getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return dbx.Instance
}

func IsUserInChat(tx *gorm.DB, chatID string, userID int64) (bool, error) {
	var count int64
	err := getDB(tx).Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND user_id = ? AND left_at IS NULL", chatID, userID).
		Count(&count).Error
	return count > 0, err
}

func GetMember(tx *gorm.DB, chatID string, userID int64) (*db_models.ConversationMember, error) {
	var member db_models.ConversationMember
	err := getDB(tx).Where("internal_chat_id = ? AND user_id = ?", chatID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

func NextLocalID(tx *gorm.DB, chatID string, fromID int64) (uint64, error) {
	var conv db_models.Conversation

	result := tx.Model(&conv).
		Where("internal_id = ?", chatID).
		Update("last_message_id", gorm.Expr("last_message_id + 1"))

	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected == 0 {
		newConv := db_models.Conversation{
			InternalID:    chatID,
			LastMessageID: 1,
		}
		if err := tx.Create(&newConv).Error; err != nil {
			return 0, err
		}
		return 1, nil
	}

	if err := tx.Select("last_message_id").First(&conv, "internal_id = ?", chatID).Error; err != nil {
		return 0, err
	}

	return conv.LastMessageID, nil
}

func LeaveChat(tx *gorm.DB, chatID string, userID int64) error {
	return getDB(tx).Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND user_id = ?", chatID, userID).
		Update("left_at", time.Now()).Error
}

func MarkAsRead(tx *gorm.DB, chatID string, userID int64, messageID uint64) error {
	return getDB(tx).Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND user_id = ?", chatID, userID).
		Where("last_read_id < ?", messageID).
		Update("last_read_id", messageID).Error
}

func GetActiveMemberIDs(tx *gorm.DB, chatID string) ([]int64, error) {
	var ids []int64
	err := getDB(tx).Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND left_at IS NULL", chatID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func GetConversation(tx *gorm.DB, chatID string) (*db_models.Conversation, error) {
	var conv db_models.Conversation
	err := tx.Where("internal_id = ?", chatID).First(&conv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

func CreateConversation(ownerID int64, userIDs []int64, isGroupChat bool) (*db_models.Conversation, error) {
	var conv db_models.Conversation
	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		var chatID string
		var targetPeerID int64
		if !isGroupChat {
			targetPeerID = userIDs[0]
			chatID = GetInternalChatID(targetPeerID, ownerID)

			existing, _ := GetConversation(tx, chatID)
			if existing != nil {
				conv = *existing
				return nil
			}
		}

		conv = db_models.Conversation{
			InternalID: chatID,
			OwnerID:    &ownerID,
			Settings:   []byte("{}"),
			CreatedAt:  time.Now(),
		}

		if err := tx.Create(&conv).Error; err != nil {
			return err
		}

		if isGroupChat {
			conv.PeerID = int64(2000000000 + conv.ID)
			conv.InternalID = "c_" + strconv.FormatInt(conv.PeerID, 10)
			tx.Model(&conv).Updates(map[string]interface{}{
				"peer_id":     conv.PeerID,
				"internal_id": conv.InternalID,
			})
			chatID = conv.InternalID
		} else {
			conv.PeerID = targetPeerID
			tx.Model(&conv).Update("peer_id", targetPeerID)
		}

		var members []db_models.ConversationMember

		addMember := func(uID int64, pID int64, isAdmin bool) {
			members = append(members, db_models.ConversationMember{
				InternalChatID: chatID,
				PeerID:         pID,
				UserID:         uID,
				IsAdmin:        isAdmin,
				JoinedAt:       time.Now(),
				InvitedBy:      ownerID,
			})
		}

		addMember(ownerID, targetPeerID, true)

		for _, uID := range userIDs {
			if uID == ownerID {
				continue
			}

			pID := ownerID
			if isGroupChat {
				pID = conv.PeerID
			}
			addMember(uID, pID, false)
		}

		return tx.Create(&members).Error
	})

	return &conv, err
}

func GetInternalChatID(peerID int64, currentUserID int64) string {
	if peerID > 2000000000 {
		return "c_" + strconv.FormatInt(peerID, 10)
	}
	if peerID < 0 || currentUserID < 0 {
		var groupID, userID int64
		if peerID < 0 {
			groupID = peerID
			userID = currentUserID
		} else {
			groupID = currentUserID
			userID = peerID
		}
		return "g" + strconv.FormatInt(groupID, 10) + "_" + strconv.FormatInt(userID, 10)
	}

	u1, u2 := currentUserID, peerID
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	return "dm" + strconv.FormatInt(u1, 10) + "_" + strconv.FormatInt(u2, 10)
}

func DerivePeerID(chatID string, currentUserID int64) int64 {
	if strings.HasPrefix(chatID, "c_") {
		id, _ := strconv.ParseInt(chatID[2:], 10, 64)
		return id
	}

	if strings.HasPrefix(chatID, "g") {
		parts := strings.Split(chatID[1:], "_")
		if len(parts) < 2 {
			return 0
		}
		id1, _ := strconv.ParseInt(parts[0], 10, 64)
		id2, _ := strconv.ParseInt(parts[1], 10, 64)

		if id1 == currentUserID {
			return id2
		}
		return id1
	}

	if strings.HasPrefix(chatID, "dm") {
		parts := strings.Split(chatID[3:], "_")
		id1, _ := strconv.ParseInt(parts[0], 10, 64)
		id2, _ := strconv.ParseInt(parts[1], 10, 64)
		if id1 == currentUserID {
			return id2
		}
		return id1
	}
	return 0
}

func RefreshChatLastMessage(tx *gorm.DB, internalChatID string) error {
	var lastMsg db_models.Message
	err := tx.Where("chat_id = ? AND (flags & 128) = 0", internalChatID).
		Order("local_id DESC").
		First(&lastMsg).Error

	var newLastID uint64 = 0
	if err == nil {
		newLastID = lastMsg.LocalID
	}

	if err := tx.Model(&db_models.Conversation{}).
		Where("internal_id = ?", internalChatID).
		Update("last_message_id", newLastID).Error; err != nil {
		return err
	}

	return tx.Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ?", internalChatID).
		Update("last_message_id", newLastID).Error
}
