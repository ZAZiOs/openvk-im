package redis_listener

import (
	"encoding/json"
	"log"
	rdm "ovk-im/src/models/redis"
	"ovk-im/src/redis"
)

func Start() {
	queueName := redis.Key("post")

	log.Printf("Redis Dispatcher started on: %s", queueName)

	for {
		result, err := redis.Client.BRPop(redis.Ctx, 0, queueName).Result()
		if err != nil {
			log.Printf("Redis Error: %v", err)
			continue
		}

		var envelope rdm.Envelope
		if err := json.Unmarshal([]byte(result[1]), &envelope); err != nil {
			log.Printf("Failed to parse Envelope: %v", err)
			continue
		}
		switch envelope.Action {
		case "msg.send":
			handleSendMsg(envelope.Data)
		default:
			log.Printf("Unknown action: %s", envelope.Action)
		}
	}
}
