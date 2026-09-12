package account

import (
	"net/http"
	"strconv"
	"time"

	"ovk-im/src/db"
	db_models "ovk-im/src/models/db"
	lp_models "ovk-im/src/models/longpoll"
	"ovk-im/src/transport/endpoints/core"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// SetSilenceMode sets silence mode for a given peer_id.
// time: -1 = forever, 0 = unmute, >0 = seconds to mute
func SetSilenceMode(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		r.Reject(c, 15, "Access denied: Unauthorized")
		return
	}
	currentUserID := val.(int64)

	peerID := r.GetInt64(c, "peer_id", 0)
	if peerID == 0 {
		chatID := r.GetInt64(c, "chat_id", 0)
		if chatID > 0 {
			peerID = 2000000000 + chatID
		}
	}
	if peerID == 0 {
		r.Reject(c, 100, "Missing or invalid peer_id parameter")
		return
	}

	muteTime := r.GetInt64(c, "time", 0)
	soundVal := r.GetInt(c, "sound", 1)
	disabledMentionsVal := r.GetInt(c, "disabled_mentions", 0)
	disabledMassMentionsVal := r.GetInt(c, "disabled_mass_mentions", 0)

	var disabledUntil int64 = 0
	if muteTime == -1 {
		disabledUntil = -1
	} else if muteTime > 0 {
		disabledUntil = time.Now().Unix() + muteTime
	}

	muteRecord := db_models.ConversationMute{
		UserID:               currentUserID,
		PeerID:               peerID,
		DisabledUntil:        disabledUntil,
		Sound:                soundVal == 1,
		DisabledMentions:     disabledMentionsVal == 1,
		DisabledMassMentions: disabledMassMentionsVal == 1,
		UpdatedAt:            time.Now(),
	}

	err := db.Instance.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "peer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"disabled_until",
			"sound",
			"disabled_mentions",
			"disabled_mass_mentions",
			"updated_at",
		}),
	}).Create(&muteRecord).Error

	if err != nil {
		r.Reject(c, 500, "Database error: "+err.Error())
		return
	}

	ev := lp_models.NotificationSetEvent{
		PeerID:        peerID,
		Sound:         uint8(soundVal),
		DisabledUntil: disabledUntil,
	}
	_ = r.LPRepo.PushEphemeralEvent(c.Request.Context(), currentUserID, "notification_set", ev)
	r.Broadcaster.Notify(currentUserID)
	r.LPRepo.Client.Publish(c.Request.Context(), "lp_updates", strconv.FormatInt(currentUserID, 10))

	c.JSON(http.StatusOK, gin.H{"response": 1})
}

// GetPushSettings returns push settings for a conversation.
func GetPushSettings(c *gin.Context, r *core.BaseHandler) {
	val, exists := c.Get("userID")
	if !exists || val == nil {
		r.Reject(c, 15, "Access denied: Unauthorized")
		return
	}
	currentUserID := val.(int64)

	peerID := r.GetInt64(c, "peer_id", 0)
	if peerID == 0 {
		chatID := r.GetInt64(c, "chat_id", 0)
		if chatID > 0 {
			peerID = 2000000000 + chatID
		}
	}

	pushSettings := FetchPushSettings(currentUserID, peerID)
	c.JSON(http.StatusOK, gin.H{"response": pushSettings})
}

// FetchPushSettings fetches VKPushSettings for a specific user and peer.
func FetchPushSettings(userID int64, peerID int64) *db_models.VKPushSettings {
	if userID == 0 || peerID == 0 {
		return &db_models.VKPushSettings{
			DisabledUntil: 0,
			Sound:         true,
		}
	}

	var mute db_models.ConversationMute
	err := db.Instance.Where("user_id = ? AND peer_id = ?", userID, peerID).First(&mute).Error
	if err != nil {
		return &db_models.VKPushSettings{
			DisabledUntil: 0,
			Sound:         true,
		}
	}

	now := time.Now().Unix()
	if mute.DisabledUntil == -1 {
		return &db_models.VKPushSettings{
			DisabledUntil:   -1,
			DisabledForever: true,
			NoSound:         !mute.Sound,
			Sound:           mute.Sound,
		}
	} else if mute.DisabledUntil > now {
		return &db_models.VKPushSettings{
			DisabledUntil: mute.DisabledUntil,
			NoSound:       !mute.Sound,
			Sound:         mute.Sound,
		}
	}

	return &db_models.VKPushSettings{
		DisabledUntil: 0,
		Sound:         true,
	}
}

// FetchPushSettingsBatch fetches VKPushSettings for multiple peers for a user.
func FetchPushSettingsBatch(userID int64, peerIDs []int64) map[int64]*db_models.VKPushSettings {
	result := make(map[int64]*db_models.VKPushSettings)
	for _, pid := range peerIDs {
		result[pid] = &db_models.VKPushSettings{
			DisabledUntil: 0,
			Sound:         true,
		}
	}

	if userID == 0 || len(peerIDs) == 0 {
		return result
	}

	var mutes []db_models.ConversationMute
	db.Instance.Where("user_id = ? AND peer_id IN (?)", userID, peerIDs).Find(&mutes)

	now := time.Now().Unix()
	for _, mute := range mutes {
		if mute.DisabledUntil == -1 {
			result[mute.PeerID] = &db_models.VKPushSettings{
				DisabledUntil:   -1,
				DisabledForever: true,
				NoSound:         !mute.Sound,
				Sound:           mute.Sound,
			}
		} else if mute.DisabledUntil > now {
			result[mute.PeerID] = &db_models.VKPushSettings{
				DisabledUntil: mute.DisabledUntil,
				NoSound:       !mute.Sound,
				Sound:         mute.Sound,
			}
		} else {
			result[mute.PeerID] = &db_models.VKPushSettings{
				DisabledUntil: 0,
				Sound:         true,
			}
		}
	}

	return result
}
