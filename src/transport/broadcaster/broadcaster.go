package broadcaster

import "sync"

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[int64][]chan struct{}
}

func New() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[int64][]chan struct{}),
	}
}

func (b *Broadcaster) Subscribe(userID int64) chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan struct{}, 1)
	b.subscribers[userID] = append(b.subscribers[userID], ch)
	return ch
}

func (b *Broadcaster) Unsubscribe(userID int64, ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[userID]
	for i, s := range subs {
		if s == ch {
			b.subscribers[userID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

func (b *Broadcaster) Notify(userID int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers[userID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
