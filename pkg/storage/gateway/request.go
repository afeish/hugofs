package gateway

import (
	"sync"
	"sync/atomic"

	"github.com/afeish/hugo/pkg/storage/adaptor"
)

var requestPool = sync.Pool{
	New: func() interface{} {
		return new(request)
	},
}

type request struct {
	// Input values
	Block []byte
	Key   adaptor.BlockKey
	Err   error
	ref   int32
	// Output values and wait group stuff below
	Future
}

func (req *request) reset() {
	req.Block = nil
	req.Key = adaptor.EmptyBlockKey
	req.Err = nil
	req.Future = nil
	req.ref = 0
}

func (req *request) IncrRef() {
	atomic.AddInt32(&req.ref, 1)
}

func (req *request) DecrRef() {
	nRef := atomic.AddInt32(&req.ref, -1)
	if nRef > 0 {
		return
	}
	req.Block = nil
	requestPool.Put(req)
}

func (req *request) Wait() error {
	req.Future.Await()
	err := req.Err
	req.DecrRef() // DecrRef after writing to DB.
	return err
}
