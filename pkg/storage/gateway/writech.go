package gateway

import (
	"context"

	"github.com/afeish/hugo/global"
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type WriteCh struct {
	adaptor.StorageBackend
	reqc      chan *request
	flushc    chan struct{}
	flushackc chan struct{}
	errc      chan error

	closer *global.Closer
	lg     *zap.Logger
}

func NewWriteCh(lg *zap.Logger, c adaptor.StorageBackend) *WriteCh {
	sc := &WriteCh{
		StorageBackend: c,
		lg:             lg,
		reqc:           make(chan *request),
		closer:         global.NewCloser(1),
		errc:           make(chan error),
		flushc:         make(chan struct{}),
		flushackc:      make(chan struct{}),
	}

	go sc.runLoop()
	return sc
}

func (c *WriteCh) sendReq(req *request) {
	c.reqc <- req
}

func (c *WriteCh) flushImmediate() {
	c.flushc <- struct{}{}
	<-c.flushackc
}

func (c *WriteCh) runLoop() {
	defer c.closer.Done()
	pendingCh := make(chan struct{}, 1)

	flushAcked := func() {
		c.flushackc <- struct{}{}
	}
	writeRequests := func(reqs []*request, isFlush bool) {
		c.lg.Debug("batch write blocks", zap.Int("batch-count", len(reqs)))
		g, gCtx := errgroup.WithContext(context.Background())
		for _, req := range reqs {
			req := req
			g.Go(func() error {
				defer func() {
					req.Future.Done()
					req.DecrRef()
				}()
				c.lg.Debug("write block", zap.String("block", req.Key.String()))
				return c.StorageBackend.Write(gCtx, req.Key, req.Block)
			})
		}
		if err := g.Wait(); err != nil {
			c.errc <- err
		}

		if isFlush {
			c.lg.Debug("batch flush blocks acked", zap.Int("batch-count", len(reqs)))
			flushAcked()
		}
		<-pendingCh
	}

	reqs := make([]*request, 0, 10)
	for {
		var r *request
		select {
		case r = <-c.reqc:
		case <-c.flushc:
			goto flushCase
		case <-c.closer.HasBeenClosed():
			goto closedCase
		}

		for {
			reqs = append(reqs, r)
			c.lg.Debug("req size", zap.Int("req-size", len(reqs)))

			if len(reqs) >= 10 {
				pendingCh <- struct{}{} // blocking.
				goto writeCase
			}

			select {
			// Either push to pending, or continue to pick from writeCh.
			case r = <-c.reqc:
			case pendingCh <- struct{}{}:
				goto writeCase
			case <-c.closer.HasBeenClosed():
				goto closedCase
			}
		}

	closedCase:
		// All the pending request are drained.
		// Don't close the writeCh, because it has be used in several places.
		for {
			select {
			case r = <-c.reqc:
				reqs = append(reqs, r)
			default:
				pendingCh <- struct{}{} // Push to pending before doing a write.
				writeRequests(reqs, false)
				return
			}
		}
	flushCase:
		go writeRequests(reqs, true)
		reqs = make([]*request, 0, 10)
	writeCase:
		go writeRequests(reqs, false)
		reqs = make([]*request, 0, 10)
	}
}

func (c *WriteCh) Close() {
	c.closer.SignalAndWait()
	close(c.reqc)
	close(c.flushc)
	close(c.errc)
	close(c.flushackc)
}
