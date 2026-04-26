package repo

import (
	"encoding/json"
	"log"
	"ovk-im/src/db"
	models "ovk-im/src/models/db"
	"ovk-im/src/transport/broadcaster"
)

func GetUpdatesForUser(userID int64, lastTS uint64) ([]models.Event, error) {
	var events []models.Event

	err := db.Instance.
		Where("user_id = ? AND id > ?", userID, lastTS).
		Order("id ASC").
		Limit(50).
		Find(&events).Error

	return events, err
}

func SendBatchEvents(userIDs []int64, eventType string, data interface{}) error {
	if len(userIDs) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	events := make([]models.Event, len(userIDs))
	for i, uid := range userIDs {
		events[i] = models.Event{
			UserID: uid,
			Type:   eventType,
			Data:   models.EncryptedJSON(jsonData),
		}
	}

	if err := db.Instance.Create(&events).Error; err != nil {
		log.Printf("Failed to batch insert events: %v", err)
		return err
	}

	for _, uid := range userIDs {
		broadcaster.NotifyUser(uid)
	}

	return nil
}
