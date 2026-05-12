package server

import "sync"

// 1 秒以内に消費できないクライアントは次回 tick の最新状態で追い付ける前提で、
// 送信は非ブロッキング・バッファ満杯時はドロップする。
type Broadcaster struct {
	mu     sync.Mutex
	subs   map[int]chan []byte
	nextID int
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[int]chan []byte{}}
}

func (b *Broadcaster) Subscribe() (int, <-chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan []byte, 8)
	b.subs[id] = ch
	return id, ch
}

func (b *Broadcaster) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		close(ch)
		delete(b.subs, id)
	}
}

func (b *Broadcaster) Publish(payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- payload:
		default:
		}
	}
}
