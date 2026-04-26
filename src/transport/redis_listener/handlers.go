package redis_listener

import (
	"encoding/json"
	"log"
	"ovk-im/src/db/repo"
	rdm "ovk-im/src/models/redis"
)

func handleSendMsg(data json.RawMessage) {
	var payload rdm.SendMsgPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("Unmarshal SendMsg: %v", err)
		return
	}

	msgs, err := repo.AcceptMsg(payload)
	if err != nil {
		log.Printf("AcceptMsg error: %v", err)
		return
	}

	uids, err := repo.GetChatMemberIDs(payload.ChatID)
	if err != nil {
		return
	}

	err = repo.SendBatchEvents(uids, "new_messages", msgs)
	if err != nil {
		log.Printf("SendBatchEvents error: %v", err)
	}
}
