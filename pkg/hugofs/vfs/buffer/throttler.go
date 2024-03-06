package buffer

import (
	"context"
	"sync"
	"sync/atomic"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	"go.uber.org/zap"
)

type Throttler struct {
	sync.RWMutex
	ctx   context.Context
	limit int

	tokenc chan struct{}

	receivec chan meta.Event
	pubsub   *Pubsub[uint64, meta.Event]

	lru *BlockLRU

	closablec ClosableCh

	availTokens atomic.Int32

	lg *zap.Logger
}

func NewThrottler(ctx context.Context, limit int, lg *zap.Logger) *Throttler {
	if limit <= 0 {
		limit = 256
	}
	receiveCh := make(chan meta.Event)

	lru, _ := NewBlockLRU(limit, func(key meta.BlockKey, value bool) {})
	l := &Throttler{
		ctx:       ctx,
		limit:     limit,
		tokenc:    make(chan struct{}, limit),
		receivec:  receiveCh,
		pubsub:    NewPubsub[uint64, meta.Event](),
		lru:       lru,
		closablec: NewClosableCh(),
		lg:        lg.Named("throttler"),
	}

	for i := 0; i < limit; i++ {
		l.tokenc <- struct{}{}
	}
	l.availTokens.Store(int32(limit))

	go l.handleEvent()

	return l
}

func (l *Throttler) Send(evt meta.Event) {
	l.receivec <- evt
}

func (l *Throttler) handleEvent() {
	for {
		select {
		case i, ok := <-l.receivec:
			if !ok {
				return
			}
			switch i.GetType() {
			case meta.EventTypeEVICTCONFIRM:
				// l.lg.Sugar().Debugf("block [ %v ] free confirmed. Remove from the linked list. Signal the acquire of the block", i.(*EvictBlockConfirmEvent).blockKey)
				if !l.closablec.IsClosed() {
					l._release()
				}
			case meta.EventTypeUPDATE:
				// l.lg.Sugar().Debugf("block [ %v ] accessed, move to the front", i.(*UpdateBlockEvent).blockKey)
				evt := i.(*meta.UpdateBlockEvent)
				l.lru.Put(evt.GetKey())
			}
		case <-l.ctx.Done():
			return
		case <-l.closablec.ClosedCh():
			return
		}
	}
}

func (l *Throttler) Register(ino uint64) <-chan meta.Event {
	l.Lock()
	defer l.Unlock()

	return l.pubsub.Subscribe(ino)
}

func (l *Throttler) UnRegister(ino uint64) {
	l.Lock()
	defer l.Unlock()

	// l.lg.Sugar().Debugf("unregister listeners for buffer [%d], active-list [%d/%d], token [%d/%d] ", ino, l.lru.Size(), l.lru.Cap(), l.availTokens.Load(), l.limit)
	l.pubsub.UnSubscribe(ino)

	if n := l.lru.RemoveByIno(ino); n > 0 {
		for i := 0; i < n; i++ {
			l._release()
		}
	}
	// l.lg.Sugar().Debugf("unregister listeners for buffer [%d] done, active-list [%d/%d], token [%d/%d] ", ino, l.lru.Size(), l.lru.Cap(), l.availTokens.Load(), l.limit)
}
func (l *Throttler) _acquire() {
	l.Lock()
	defer l.Unlock()

	select {
	case <-l.tokenc:
		l.availTokens.Add(-1)
	case <-l.closablec.ClosedCh():
		return
	}
}

func (l *Throttler) TryAcquire(key meta.BlockKey) bool {
	l.Lock()
	defer l.Unlock()

	// l.lg.Sugar().Debugf("acquiring lock for block [%s], active-list [%d/%d], token [%d/%d] begin", key.String(), l.lru.Size(), l.lru.Cap(), l.availTokens.Load(), l.limit)

	select {
	case <-l.tokenc:
		l._occupy(key)
		return true
	default:
	}

	removed, ok := l.lru.RemoveOldest()
	if !ok {
		return false
	}

	// l.lg.Sugar().Debugf("acquiring lock for block [%s], evict block [%s], active-list [%d/%d], token [%d/%d] processing", key.String(), removed.String(), l.lru.Size(), l.lru.Cap(), l.availTokens.Load(), l.limit)
	l.pubsub.Publish(removed.GetIno(), meta.NewEvictBlockReqEvent(removed))

	for {
		select {
		case <-l.tokenc:
			l._occupy(key)
			return true
		case <-l.closablec.ClosedCh():
			return false
		}
	}
}

func (l *Throttler) _occupy(key meta.BlockKey) {
	l.lru.Put(key)
	l.availTokens.Add(-1)
}

func (l *Throttler) _release() {
	l.tokenc <- struct{}{}
	l.availTokens.Add(1)
}

func (l *Throttler) IsFull() bool {
	return l.limit == int(l.availTokens.Load())
}

func (l *Throttler) GetLimit() int {
	return l.limit
}

func (l *Throttler) Shutdown() {
	l.closablec.Close(func() {
		close(l.tokenc)
		l.Lock()
		l.pubsub.Close()
		l.Unlock()
		close(l.receivec)
	})
}
