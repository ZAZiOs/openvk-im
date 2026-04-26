package broadcaster

import (
	"sync"
)

var (
	subscribers = make(map[int64][]chan struct{})
	mu          sync.RWMutex
)

func Subscribe(userID int64, ch chan struct{}) {
	mu.Lock()
	defer mu.Unlock()
	subscribers[userID] = append(subscribers[userID], ch)
}

func Unsubscribe(userID int64, ch chan struct{}) {
	mu.Lock()
	defer mu.Unlock()
	subs := subscribers[userID]
	for i, v := range subs {
		if v == ch {
			subscribers[userID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func NotifyUser(userID int64) {
	mu.RLock()
	defer mu.RUnlock()

	if channels, ok := subscribers[userID]; ok {
		for _, ch := range channels {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}
