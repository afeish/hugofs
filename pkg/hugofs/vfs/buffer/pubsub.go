package buffer

import "sync"

type Pubsub[K comparable, T any] struct {
	mu     sync.RWMutex
	subs   map[K][]chan T
	closed bool
}

func NewPubsub[K comparable, T any]() *Pubsub[K, T] {
	ps := &Pubsub[K, T]{}
	ps.subs = make(map[K][]chan T)
	return ps
}

func (ps *Pubsub[K, T]) Subscribe(topic K) <-chan T {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan T, 1)
	ps.subs[topic] = append(ps.subs[topic], ch)
	return ch
}

func (ps *Pubsub[K, T]) UnSubscribe(topic K) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	chs, found := ps.subs[topic]
	if !found {
		return
	}
	delete(ps.subs, topic)
	for _, ch := range chs {
		close(ch)
	}
}

func (ps *Pubsub[K, T]) Publish(topic K, msg T) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return
	}

	for _, ch := range ps.subs[topic] {
		ch <- msg
	}
}

func (ps *Pubsub[K, T]) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.closed {
		ps.closed = true
		for _, subs := range ps.subs {
			for _, ch := range subs {
				close(ch)
			}
		}
	}
}
