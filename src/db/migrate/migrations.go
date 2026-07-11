package db_migrate

import (
	"fmt"
	"log"
	env "ovk-im/src/config"
	dbx "ovk-im/src/db"
	dbm "ovk-im/src/models/db"
	"ovk-im/src/repo/chat"
	"ovk-im/src/repo/search"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func CreateOrMigrateDB() {
	err := dbx.Instance.AutoMigrate(
		&dbm.Conversation{},
		&dbm.Message{},
		&dbm.ConversationMember{},
		&dbm.ChatInvite{},
		&dbm.ImState{},
		&dbm.MessageSearchIndex{},
		&dbm.ImportantMessage{},
	)

	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("\n\nDatabase initialized")
}

type LegacyMessage struct {
	ID            int64  `gorm:"column:id;primaryKey"`
	SenderType    string `gorm:"column:sender_type"`
	SenderID      uint64 `gorm:"column:sender_id"`
	RecipientType string `gorm:"column:recipient_type"`
	RecipientID   uint64 `gorm:"column:recipient_id"`
	Content       string `gorm:"column:content"`
	Created       int64  `gorm:"column:created"`
	Edited        int64  `gorm:"column:edited"`
	Ad            bool   `gorm:"column:ad"`
	Deleted       bool   `gorm:"column:deleted"`
	Unread        bool   `gorm:"column:unread"`
}

func (LegacyMessage) TableName() string {
	return "messages"
}

func MigrateFromLegacy() {
	log.Println("Starting experimental migration from OpenVK legacy messages...")

	user := env.Get("DB_USER", "root")
	pass := env.Get("DB_PASS", "")
	host := env.Get("DB_HOST", "127.0.0.1")
	port := env.Get("DB_PORT", "3306")

	legacyDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, "openvk")
	legacyDB, err := gorm.Open(mysql.Open(legacyDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to legacy database: %v", err)
	}

	if dbx.Instance == nil {
		dbx.Connect()
	}

	dbx.Instance.Logger = dbx.Instance.Logger.LogMode(logger.Silent)

	secret := env.Get("SECRET_KEY", "error")
	if len(secret) < 64 {
		log.Println("SECRET_KEY .env variable is less than 64 bytes")
		return
	}
	searchRepo := search.NewRepository(dbx.Instance, []byte(secret))

	localIDCache := make(map[string]uint64)
	memberCache := make(map[string]bool)

	var legacyMessages []LegacyMessage
	batchSize := 2000

	result := legacyDB.FindInBatches(&legacyMessages, batchSize, func(tx *gorm.DB, batch int) error {
		return dbx.Instance.Transaction(func(targetTx *gorm.DB) error {
			var messagesToInsert []dbm.Message
			var indexesToInsert []dbm.MessageSearchIndex
			var membersToUpsert []dbm.ConversationMember

			touchedChatsInBatch := make(map[string]bool)

			var missingChatIDs []string
			for _, msg := range legacyMessages {
				currentUserID := int64(msg.SenderID)
				peerID := int64(msg.RecipientID)
				chatID := chat.GetInternalChatID(peerID, currentUserID)
				if _, ok := localIDCache[chatID]; !ok {
					missingChatIDs = append(missingChatIDs, chatID)
					localIDCache[chatID] = 0 // Временная заглушка
				}
			}

			if len(missingChatIDs) > 0 {
				type Result struct {
					ChatID string
					MaxID  uint64
				}
				var results []Result
				targetTx.Model(&dbm.Message{}).
					Select("chat_id, COALESCE(max(local_id), 0) as max_id").
					Where("chat_id IN ?", missingChatIDs).
					Group("chat_id").
					Scan(&results)

				for _, r := range results {
					localIDCache[r.ChatID] = r.MaxID
				}
			}

			for _, msg := range legacyMessages {
				currentUserID := int64(msg.SenderID)
				peerID := int64(msg.RecipientID)
				chatID := chat.GetInternalChatID(peerID, currentUserID)

				localIDCache[chatID]++
				currentLocalID := localIDCache[chatID]
				touchedChatsInBatch[chatID] = true

				usersToCheck := []int64{currentUserID}
				if peerID < 2000000000 && peerID != currentUserID {
					usersToCheck = append(usersToCheck, peerID)
				}

				for _, uid := range usersToCheck {
					cacheKey := fmt.Sprintf("%s_%d", chatID, uid)
					if !memberCache[cacheKey] {
						membersToUpsert = append(membersToUpsert, dbm.ConversationMember{
							InternalChatID: chatID,
							UserID:         uid,
							JoinedAt:       time.Unix(msg.Created, 0),
							IsAdmin:        true,
							LastMessageID:  currentLocalID,
						})
						memberCache[cacheKey] = true
					}
				}

				msgTime := time.Unix(msg.Created, 0)
				var updatedTime *time.Time
				if msg.Edited > 0 {
					t := time.Unix(msg.Edited, 0)
					updatedTime = &t
				}

				messagesToInsert = append(messagesToInsert, dbm.Message{
					ChatID:    chatID,
					LocalID:   currentLocalID,
					FromID:    currentUserID,
					Text:      dbm.EncryptedJSON(msg.Content),
					CreatedAt: msgTime,
					EditedAt:  updatedTime,
				})
			}

			if len(membersToUpsert) > 0 {
				if err := targetTx.Clauses(clause.OnConflict{DoNothing: true}).Create(&membersToUpsert).Error; err != nil {
					return err
				}
			}

			if err := targetTx.Create(&messagesToInsert).Error; err != nil {
				return err
			}

			for i, m := range messagesToInsert {
				legacyMsg := legacyMessages[i]
				if !legacyMsg.Deleted && legacyMsg.Content != "" {
					idx := searchRepo.GenerateBlindIndexes(m.ID, m.ChatID, string(legacyMsg.Content))
					indexesToInsert = append(indexesToInsert, idx...)
				}
			}
			if len(indexesToInsert) > 0 {
				if err := targetTx.Create(&indexesToInsert).Error; err != nil {
					return err
				}
			}

			for chatID := range touchedChatsInBatch {
				lastID := localIDCache[chatID]

				targetTx.Model(&dbm.ConversationMember{}).
					Where("internal_chat_id = ?", chatID).
					Update("last_message_id", lastID)

				err := targetTx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "internal_id"}},
					DoUpdates: clause.Assignments(map[string]interface{}{"last_message_id": lastID}),
				}).Create(&dbm.Conversation{
					InternalID:    chatID,
					LastMessageID: lastID,
				}).Error
				if err != nil {
					return err
				}
			}

			log.Printf("Batch %d processed", batch)
			return nil
		})
	})

	if result.Error != nil {
		log.Printf("Migration error: %v", result.Error)
	} else {
		log.Println("Migration finished successfully!")
	}
}
