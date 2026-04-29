package chat

import (
	"errors"
	dbx "ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	"time"

	"gorm.io/gorm"
)

func getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return dbx.Instance
}

func IsUserInChat(tx *gorm.DB, peerID int64, userID int64) (bool, error) {
	var count int64
	err := getDB(tx).Model(&db_models.ConversationMember{}).
		Where("peer_id = ? AND user_id = ? AND left_at IS NULL", peerID, userID).
		Count(&count).Error
	return count > 0, err
}

func GetMember(tx *gorm.DB, peerID int64, userID int64) (*db_models.ConversationMember, error) {
	var member db_models.ConversationMember
	err := getDB(tx).Where("peer_id = ? AND user_id = ?", peerID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

func NextLocalID(tx *gorm.DB, peerID int64, fromID int64) (uint64, error) {
	var conv db_models.Conversation
	result := tx.Model(&conv).Where("peer_id = ?", peerID).Update("last_message_id", gorm.Expr("last_message_id + 1"))

	if result.RowsAffected == 0 {
		if fromID < 0 && peerID < 2000000000 {
			return 0, errors.New("community can not start conversation")
		}
		if peerID < 2000000000 {
			newConv := db_models.Conversation{PeerID: peerID, LastMessageID: 1}
			if err := tx.Create(&newConv).Error; err != nil {
				return 0, err
			}
			return 1, nil
		}
		return 0, errors.New("conversation not found")
	}

	tx.Where("peer_id = ?", peerID).Select("last_message_id").First(&conv)

	return conv.LastMessageID, nil
}

func LeaveChat(tx *gorm.DB, peerID int64, userID int64) error {
	return getDB(tx).Model(&db_models.ConversationMember{}).
		Where("peer_id = ? AND user_id = ?", peerID, userID).
		Update("left_at", time.Now()).Error
}

func MarkAsRead(tx *gorm.DB, peerID int64, userID int64, messageID uint64) error {
	return getDB(tx).Model(&db_models.ConversationMember{}).
		Where("peer_id = ? AND user_id = ?", peerID, userID).
		Update("last_read_id", messageID).Error
}

func GetActiveMemberIDs(tx *gorm.DB, peerID int64) ([]int64, error) {
	var ids []int64
	err := getDB(tx).Model(&db_models.ConversationMember{}).
		Where("peer_id = ? AND left_at IS NULL", peerID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func GetConversation(tx *gorm.DB, peerID int64) (*db_models.Conversation, error) {
	var conv db_models.Conversation
	err := tx.Where("peer_id = ?", peerID).First(&conv).Error
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
		var targetPeerID int64
		if !isGroupChat {
			if len(userIDs) == 0 {
				return errors.New("receiver id required for DM")
			}
			targetPeerID = userIDs[0]

			existing, _ := GetConversation(tx, targetPeerID)
			if existing != nil {
				conv = *existing
				return nil
			}
		}

		conv = db_models.Conversation{
			PeerID:   targetPeerID,
			OwnerID:  &ownerID,
			Settings: []byte("{}"),
		}

		if err := tx.Create(&conv).Error; err != nil {
			return err
		}

		if isGroupChat {
			conv.PeerID = int64(2000000000 + conv.ID)
			if err := tx.Model(&conv).Update("peer_id", conv.PeerID).Error; err != nil {
				return err
			}
		}

		var members []db_models.ConversationMember

		members = append(members, db_models.ConversationMember{
			PeerID:    conv.PeerID,
			UserID:    ownerID,
			IsAdmin:   true,
			JoinedAt:  time.Now(),
			InvitedBy: ownerID,
		})

		for _, uID := range userIDs {
			if uID == ownerID {
				continue
			}
			members = append(members, db_models.ConversationMember{
				PeerID:    conv.PeerID,
				UserID:    uID,
				IsAdmin:   false,
				JoinedAt:  time.Now(),
				InvitedBy: ownerID,
			})
		}

		if err := tx.Create(&members).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &conv, nil
}
