package meta

import (
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore

	"github.com/rfyiamcool/go-timewheel"

	"github.com/samber/mo"
)

// ttlEntry is a cache entry with a TTL.
type ttlEntry struct {
	ino        Ino
	rwbuffer   RWBuffer
	forever    bool
	expireTime time.Time
	task       *timewheel.Task
}

// RWBufferCache is a cache for file data.
type RWBufferCache struct {
	mu       sync.RWMutex
	ino2data map[Ino]*ttlEntry
	tw       *timewheel.TimeWheel
	opt      *option
}

type option struct {
	defaultTTL    time.Duration
	checkInterval time.Duration
}

func NewRWBufferCache() *RWBufferCache {
	tw, err := timewheel.NewTimeWheel(1*time.Second, 360)
	if err != nil {
		panic(err)
	}

	tw.Start()
	return &RWBufferCache{
		ino2data: make(map[Ino]*ttlEntry),
		tw:       tw,
		opt:      &option{defaultTTL: time.Minute * 1, checkInterval: time.Second * 5},
	}
}

func (c *RWBufferCache) Get(in Ino) mo.Option[RWBuffer] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.ino2data[in]

	if !ok || (!e.forever && time.Since(e.expireTime) > 0) {
		return mo.None[RWBuffer]()
	}

	return mo.Some(e.rwbuffer)

}
func (c *RWBufferCache) Put(in Ino, rwbuffer RWBuffer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl, found := c.ino2data[in]; found {
		c.tw.Remove(ttl.task)
		ttl.rwbuffer.Destroy()
	}

	c.ino2data[in] = &ttlEntry{
		ino:        in,
		rwbuffer:   rwbuffer,
		expireTime: time.Now().Add(c.opt.defaultTTL),
		task: c.tw.AddCron(c.opt.checkInterval, func() {
			c._delete(in)
		}),
	}
}

func (c *RWBufferCache) _delete(in Ino) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.ino2data[in]
	if ok {
		if entry.rwbuffer.Idle(time.Minute) {
			c.tw.Remove(entry.task)
			delete(c.ino2data, in)
			entry.rwbuffer.Destroy()
		} else {
			entry.expireTime = entry.expireTime.Add(c.opt.defaultTTL)
		}
	}
}

func (c *RWBufferCache) Destroy(in Ino) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.ino2data[in]
	if ok {
		c.tw.Remove(entry.task)
		delete(c.ino2data, in)
		entry.rwbuffer.Destroy()
	}
}
