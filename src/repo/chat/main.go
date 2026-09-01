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
	var maxID uint64

	err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("internal_id = ?", chatID).
		First(&conv).Error

	isNewConv := false
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			isNewConv = true
		} else {
			return 0, err
		}
	}

	err = tx.Table("messages").
		Unscoped().
		Where("chat_id = ?", chatID).
		Select("COALESCE(MAX(local_id), 0)").
		Row().
		Scan(&maxID)

	if err != nil {
		return 0, err
	}

	nextID := maxID + 1

	if isNewConv {
		newConv := db_models.Conversation{
			InternalID:    chatID,
			LastMessageID: nextID,
		}
		if err := tx.Create(&newConv).Error; err != nil {
			return 0, err
		}
	} else {
		err = tx.Model(&conv).Update("last_message_id", nextID).Error
		if err != nil {
			return 0, err
		}
	}

	return nextID, nil
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

func CreateConversation(ownerID int64, userIDs []int64, groupTitle string) (*db_models.Conversation, error) {
	var conv db_models.Conversation
	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		conv = db_models.Conversation{
			OwnerID:   &ownerID,
			Title:     groupTitle,
			Settings:  []byte("{}"),
			CreatedAt: time.Now(),
		}

		if err := tx.Create(&conv).Error; err != nil {
			return err
		}

		conv.InternalID = "c" + strconv.FormatUint(conv.ID, 10)

		if err := tx.Table("conversations").Where("id = ?", conv.ID).Updates(map[string]interface{}{
			"internal_id": conv.InternalID,
		}).Error; err != nil {
			return err
		}

		chatID := conv.InternalID

		var members []db_models.ConversationMember
		var periods []db_models.ConversationMemberPeriod

		addMember := func(uID int64, isAdmin bool) {
			members = append(members, db_models.ConversationMember{
				InternalChatID: chatID,
				UserID:         uID,
				IsAdmin:        isAdmin,
				JoinedAt:       time.Now(),
				InvitedBy:      ownerID,
			})
			periods = append(periods, db_models.ConversationMemberPeriod{
				InternalChatID: chatID,
				UserID:         uID,
				StartLocalID:   1,
				EndLocalID:     nil,
			})
		}

		addMember(ownerID, true)

		for _, uID := range userIDs {
			if uID == ownerID {
				continue
			}
			addMember(uID, false)
		}

		if err := tx.Create(&periods).Error; err != nil {
			return err
		}

		return tx.Create(&members).Error
	})

	return &conv, err
}

func AddUserToConversation(chatID string, userID int64, inviterID int64, text string, action string, actionMid int64, actionText string) (*db_models.Message, error) {
	if !strings.HasPrefix(chatID, "c") {
		return nil, errors.New("cannot add user to direct message")
	}

	if actionMid == 0 {
		actionMid = inviterID
	}

	var msg *db_models.Message

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		var member db_models.ConversationMember
		findErr := tx.Where("internal_chat_id = ? AND user_id = ?", chatID, userID).First(&member).Error

		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		if findErr == nil && member.LeftAt == nil {
			return errors.New("user is already in the conversation")
		}

		isNewMember := errors.Is(findErr, gorm.ErrRecordNotFound)

		localID, err := NextLocalID(tx, chatID, inviterID)
		if err != nil {
			return err
		}

		msg = &db_models.Message{
			FromID:     inviterID,
			ChatID:     chatID,
			LocalID:    localID,
			Text:       db_models.EncryptedJSON(text),
			Action:     action,
			ActionMid:  actionMid,
			ActionText: actionText,
			CreatedAt:  time.Now(),
		}

		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		// Create a new active period starting from this service message
		newPeriod := db_models.ConversationMemberPeriod{
			InternalChatID: chatID,
			UserID:         userID,
			StartLocalID:   localID,
			EndLocalID:     nil,
		}
		if err := tx.Create(&newPeriod).Error; err != nil {
			return err
		}

		if isNewMember {
			newMember := db_models.ConversationMember{
				InternalChatID: chatID,
				UserID:         userID,
				IsAdmin:        false,
				JoinedAt:       time.Now(),
				InvitedBy:      inviterID,
				StartMessageID: uint64(msg.ID),
				LastMessageID:  localID,
			}
			if err := tx.Create(&newMember).Error; err != nil {
				return err
			}
		} else {
			err = tx.Model(&db_models.ConversationMember{}).
				Where("internal_chat_id = ? AND user_id = ?", chatID, userID).
				Updates(map[string]interface{}{
					"left_at":          gorm.Expr("NULL"),
					"invited_by":       inviterID,
					"joined_at":        time.Now(),
					"start_message_id": uint64(msg.ID),
					"last_message_id":  localID,
				}).Error
			if err != nil {
				return err
			}
		}

		err = tx.Table("conversation_members").
			Where("internal_chat_id = ? AND left_at IS NULL AND user_id != ?", chatID, userID).
			Update("last_message_id", localID).Error

		return err
	})

	if err != nil {
		return nil, err
	}

	return msg, nil
}

func RemoveUserFromConversation(tx *gorm.DB, chatID string, userID int64) error {
	return getDB(tx).Model(&db_models.ConversationMember{}).
		Where("internal_chat_id = ? AND user_id = ?", chatID, userID).
		Update("left_at", time.Now()).Error
}

func CloseActiveMemberPeriod(tx *gorm.DB, chatID string, userID int64, endLocalID uint64) error {
	return getDB(tx).Model(&db_models.ConversationMemberPeriod{}).
		Where("internal_chat_id = ? AND user_id = ? AND end_local_id IS NULL", chatID, userID).
		Update("end_local_id", endLocalID).Error
}

func EnsureMemberPeriod(tx *gorm.DB, chatID string, userID int64, startLocalID uint64) error {
	var count int64
	err := getDB(tx).Model(&db_models.ConversationMemberPeriod{}).
		Where("internal_chat_id = ? AND user_id = ?", chatID, userID).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return getDB(tx).Create(&db_models.ConversationMemberPeriod{
			InternalChatID: chatID,
			UserID:         userID,
			StartLocalID:   startLocalID,
			EndLocalID:     nil,
		}).Error
	}
	return nil
}

/*
Мини документация по сервисным сообщениям:
chat_create       - Создана беседа пользователем (fromID & Mid)
chat_title_update - Обновлено название беседы пользователем (fromID & Mid)
chat_photo_remove - Удалена фотка беседы пользователем (fromID & Mid)
chat_photo_update - Обновлена фотка беседы пользователем (fromID & Mid)
chat_invite_user  - Пользователем fromID приглашём пользователь Mid
chat_kick_user    - Пользователем fromID исключён пользователь Mid
chat_pin_message  - Пользователь fromID закрепил сообщение Mid
chat_unpin_message - Пользователь fromID открепил сообщение Mid
chat_invite_user_by_link - Пользователь (fromID & Mid) зашёл в чат по ссылке приглашению
*/

func CreateServiceMessage(fromID int64, chatID string, text string, action string, actionMid int64, actionText string) (*db_models.Message, error) {
	if actionMid == 0 {
		actionMid = fromID
	}

	var msg *db_models.Message

	err := dbx.Instance.Transaction(func(tx *gorm.DB) error {
		localID, err := NextLocalID(tx, chatID, fromID)
		if err != nil {
			return err
		}

		msg = &db_models.Message{
			FromID:     fromID,
			ChatID:     chatID,
			LocalID:    localID,
			Text:       db_models.EncryptedJSON(text),
			Action:     action,
			ActionMid:  actionMid,
			ActionText: actionText,
			CreatedAt:  time.Now(),
		}

		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		err = tx.Table("conversation_members").
			Where("internal_chat_id = ? AND left_at IS NULL", chatID).
			Update("last_message_id", localID).Error

		return err
	})

	if err != nil {
		return nil, err
	}

	return msg, nil
}

func GetInternalChatID(peerID int64, currentUserID int64) string {
	if peerID > 2000000000 {
		return "c" + strconv.FormatInt(peerID-2000000000, 10)
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

func ResolveChatID(chatIDParam string, peerID int64, userID int64, currentUserID int64) string {
	if strings.HasPrefix(chatIDParam, "c") || strings.HasPrefix(chatIDParam, "dm") || strings.HasPrefix(chatIDParam, "g") {
		return chatIDParam
	}
	if currentUserID == 0 && userID != 0 && peerID != 0 {
		return GetInternalChatID(peerID, userID)
	}
	if peerID != 0 {
		return GetInternalChatID(peerID, currentUserID)
	}
	if chatIDParam != "" {
		if id, err := strconv.ParseInt(chatIDParam, 10, 64); err == nil && id > 0 {
			return GetInternalChatID(2000000000+id, currentUserID)
		}
	}
	if userID != 0 {
		return GetInternalChatID(userID, currentUserID)
	}
	return ""
}


func DerivePeerID(chatID string, currentUserID int64) int64 {
	if strings.HasPrefix(chatID, "c") {
		id, _ := strconv.ParseInt(chatID[1:], 10, 64)
		return id + 2000000000
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
		parts := strings.Split(chatID[2:], "_")
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
