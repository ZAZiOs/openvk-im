package search

import (
	"crypto/hmac"
	"crypto/sha256"
	"strings"

	dbm "ovk-im/src/models/db"

	"gorm.io/gorm"
)

type Repository struct {
	db        *gorm.DB
	searchKey []byte // Ключ для HMAC (Blind Index)
}

func NewRepository(db *gorm.DB, searchKey []byte) *Repository {
	return &Repository{
		db:        db,
		searchKey: searchKey,
	}
}

func (r *Repository) hashWord(word string) []byte {
	h := hmac.New(sha256.New, r.searchKey)
	h.Write([]byte(strings.ToLower(word)))
	return h.Sum(nil)
}

func (r *Repository) GenerateBlindIndexes(messageID uint64, chatID int64, text string) []dbm.MessageSearchIndex {
	words := strings.Fields(text)
	var indexes []dbm.MessageSearchIndex

	seen := make(map[string]bool)
	for _, word := range words {
		word = strings.ToLower(word)
		if seen[word] || len(word) < 2 {
			continue
		}

		indexes = append(indexes, dbm.MessageSearchIndex{
			MessageID: messageID,
			ChatID:    chatID,
			WordHash:  r.hashWord(word),
		})
		seen[word] = true
	}
	return indexes
}

func (r *Repository) SearchMessages(chatID int64, query string) ([]uint64, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil, nil
	}

	var hashes [][]byte
	seen := make(map[string]bool)

	for _, word := range words {
		word = strings.ToLower(word)
		if !seen[word] {
			hashes = append(hashes, r.hashWord(word))
			seen[word] = true
		}
	}

	var messageIDs []uint64

	err := r.db.Model(&dbm.MessageSearchIndex{}).
		Select("message_id").
		Where("chat_id = ? AND word_hash IN ?", chatID, hashes).
		Group("message_id").
		Having("COUNT(DISTINCT word_hash) = ?", len(hashes)).
		Pluck("message_id", &messageIDs).Error

	if err != nil {
		return nil, err
	}

	return messageIDs, nil
}

func (r *Repository) PrepareHashes(query string) [][]byte {
	words := strings.Fields(query)
	var hashes [][]byte
	seen := make(map[string]bool)
	for _, word := range words {
		word = strings.ToLower(word)
		if !seen[word] && len(word) >= 2 {
			hashes = append(hashes, r.hashWord(word))
			seen[word] = true
		}
	}
	return hashes
}

func (r *Repository) WordsCount(query string) int {
	return len(r.PrepareHashes(query))
}
