package global

import (
	"context"
	"sync"
	"sync/atomic"
)

type Closable interface {
	Close(...func() error) error
}

var dummyCloserChan <-chan struct{}

// Closer holds the two things we need to close a goroutine and wait for it to
// finish: a chan to tell the goroutine to shut down, and a WaitGroup with
// which to wait for it to finish shutting down.
type Closer struct {
	waiting sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

// NewCloser constructs a new Closer, with an initial count on the WaitGroup.
func NewCloser(initial int) *Closer {
	ret := &Closer{}
	ret.ctx, ret.cancel = context.WithCancel(context.Background())
	ret.waiting.Add(initial)
	return ret
}

// AddRunning Add()'s delta to the WaitGroup.
func (lc *Closer) AddRunning(delta int) {
	lc.waiting.Add(delta)
}

// Ctx can be used to get a context, which would automatically get cancelled when Signal is called.
func (lc *Closer) Ctx() context.Context {
	if lc == nil {
		return context.Background()
	}
	return lc.ctx
}

// Signal signals the HasBeenClosed signal.
func (lc *Closer) Signal() {
	// Todo(ibrahim): Change Signal to return error on next badger breaking change.
	lc.cancel()
}

// HasBeenClosed gets signaled when Signal() is called.
func (lc *Closer) HasBeenClosed() <-chan struct{} {
	if lc == nil {
		return dummyCloserChan
	}
	return lc.ctx.Done()
}

// Done calls Done() on the WaitGroup.
func (lc *Closer) Done() {
	if lc == nil {
		return
	}
	lc.waiting.Done()
}

// Wait waits on the WaitGroup. (It waits for NewCloser's initial value, AddRunning, and Done
// calls to balance out.)
func (lc *Closer) Wait() {
	lc.waiting.Wait()
}

// SignalAndWait calls Signal(), then Wait().
func (lc *Closer) SignalAndWait() {
	lc.Signal()
	lc.Wait()
}

type ClosableCh struct {
	ch     chan struct{}
	closed int32

	mu sync.Mutex
}

func NewClosableCh() ClosableCh {
	return ClosableCh{
		ch: make(chan struct{}),
	}
}

func (c *ClosableCh) Close(calls ...func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.IsClosed() {
		return
	}

	for _, call := range calls {
		call()
	}

	close(c.ch)
	atomic.StoreInt32(&c.closed, 1)
}

func (c *ClosableCh) IsClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

func (c *ClosableCh) ClosedCh() chan struct{} {
	return c.ch
}
