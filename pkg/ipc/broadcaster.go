package ipc

import (
	"fsd/internal/config"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Maps the broadcast channel to the identifier.
type SubscriberMap map[chan Message]string

type Broadcaster struct {
	subscribers SubscriberMap
	lock        sync.RWMutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(SubscriberMap),
	}
}

// Subscribe adds a new subscriber channel to the broadcaster which can
// listen for any message type
func (b *Broadcaster) Subscribe(identifier string) chan Message {
	ch := make(chan Message, config.GetConfig().BroadcastBufferDepth)
	b.lock.Lock()
	defer b.lock.Unlock()
	b.subscribers[ch] = identifier
	return ch
}

// Unsubscribe removes a subscriber channel from the broadcaster
func (b *Broadcaster) Unsubscribe(ch chan Message) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
}

// Broadcast sends a broadcast message to all subscriber channels
func (b *Broadcaster) Broadcast(msg Message) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	for ch, receiver := range b.subscribers {
		// Send the message to everyone
		select {
		case ch <- msg:
		case <-time.After(1 * time.Second):
			// default allows for non-blocking I/O, but we still want to warn for lag
			zap.L().Warn("Subscriber dealyed in processing for over a second!", zap.String("receiver", receiver))
		default:
		}
	}
}
