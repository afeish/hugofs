package mem

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	used  int64
	pools []*sync.Pool
)

const (
	min_size = 1024
)

func bitCount(size int) (count int) {
	for ; size > min_size; count++ {
		size = (size + 1) >> 1
	}
	return
}

func init() {
	// 1KB ~ 256MB
	pools = make([]*sync.Pool, bitCount(1024*1024*256))
	for i := 0; i < len(pools); i++ {
		slotSize := 1024 << i
		pools[i] = &sync.Pool{
			New: func() interface{} {
				buffer := make([]byte, slotSize)
				return &buffer
			},
		}
	}

	go func() {
		for {
			time.Sleep(time.Minute * 10)
			runtime.GC()
		}
	}()
}

func getSlotPool(size int) (*sync.Pool, bool) {
	index := bitCount(size)
	if index >= len(pools) {
		return nil, false
	}
	return pools[index], true
}

func Allocate(size int) []byte {
	if pool, found := getSlotPool(size); found {
		slab := *pool.Get().(*[]byte)
		atomic.AddInt64(&used, int64(cap(slab)))
		r := slab[:size]
		zero(r)
		return r
	}
	return make([]byte, size)
}

func Free(buf []byte) {
	zero(buf)
	if pool, found := getSlotPool(cap(buf)); found {
		atomic.AddInt64(&used, -int64(cap(buf)))
		pool.Put(&buf)
	}
}

func zero(p []byte) {
	for i := range p {
		p[i] = 0
	}
}

func AllocMemory() int64 {
	return atomic.LoadInt64(&used)
}

func Align(p []byte, size int64) []byte {
	dst := make([]byte, size)
	copy(dst, p)
	return dst
}

const (
	len8 int = 0xFFFFFFF8
)

func IsAllBytesZero(data []byte) bool {
	n := len(data)

	// Magic to get largest length which could be divided by 8.
	nlen8 := n & len8
	i := 0

	for ; i < nlen8; i += 8 {
		b := *(*uint64)(unsafe.Pointer(&data[i]))
		if b != 0 {
			return false
		}
	}

	for ; i < n; i++ {
		if data[i] != 0 {
			return false
		}
	}

	return true
}
