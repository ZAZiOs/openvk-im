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

	secret := env.Get("SECRET_KEY", "your-fallback-very-long-secret-key")
	searchRepo := search.NewRepository(dbx.Instance, []byte(secret))

	var legacyMessages []LegacyMessage
	batchSize := 1000

	result := legacyDB.FindInBatches(&legacyMessages, batchSize, func(tx *gorm.DB, batch int) error {
		log.Printf("Processing batch %d...", batch)

		return dbx.Instance.Transaction(func(targetTx *gorm.DB) error {
			for _, msg := range legacyMessages {
				if err := processMessage(targetTx, msg, searchRepo); err != nil {
					log.Printf("[Import Error] Message ID %d: %v", msg.ID, err)
					continue
				}
			}
			return nil
		})
	})

	if result.Error != nil {
		log.Printf("Migration error: %v", result.Error)
	} else {
		log.Println("Migration finished successfully!")
	}
}

func processMessage(tx *gorm.DB, msg LegacyMessage, searchRepo *search.Repository) error {
	currentUserID := int64(msg.SenderID)
	if msg.SenderType != "user" {
		currentUserID = int64(msg.SenderID)
	}

	peerID := int64(msg.RecipientID)
	if msg.RecipientType != "user" {
		peerID = int64(msg.RecipientID)
	}

	internalChatID := chat.GetInternalChatID(peerID, currentUserID)

	localID, err := chat.NextLocalID(tx, internalChatID, currentUserID)
	if err != nil {
		return err
	}

	if peerID < 2000000000 {
		tx.Where(dbm.ConversationMember{InternalChatID: internalChatID, UserID: currentUserID}).
			FirstOrCreate(&dbm.ConversationMember{
				InternalChatID: internalChatID,
				PeerID:         peerID,
				UserID:         currentUserID,
				JoinedAt:       time.Unix(msg.Created, 0),
				IsAdmin:        true,
			})

		if peerID != currentUserID {
			tx.Where(dbm.ConversationMember{InternalChatID: internalChatID, UserID: peerID}).
				FirstOrCreate(&dbm.ConversationMember{
					InternalChatID: internalChatID,
					PeerID:         currentUserID,
					UserID:         peerID,
					JoinedAt:       time.Unix(msg.Created, 0),
				})
		}
	}

	zero := uint64(0)
	newMessage := dbm.Message{
		ChatID:      internalChatID,
		LocalID:     localID,
		FromID:      currentUserID,
		ReplyTo:     &zero,
		Text:        dbm.EncryptedJSON(msg.Content),
		Attachments: dbm.EncryptedJSON(""),
		CreatedAt:   time.Unix(msg.Created, 0),
	}

	if err := tx.Create(&newMessage).Error; err != nil {
		return err
	}

	tx.Model(&dbm.ConversationMember{}).
		Where("internal_chat_id = ?", internalChatID).
		Update("last_message_id", localID)

	if !msg.Deleted && msg.Content != "" {
		indexes := searchRepo.GenerateBlindIndexes(newMessage.ID, internalChatID, msg.Content)
		if len(indexes) > 0 {
			tx.Create(&indexes)
		}
	}

	log.Printf("Succesfully migrated: %s - %d\n", internalChatID, msg.ID)

	return nil
}
