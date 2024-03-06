package buffer

import (
	"context"
	"sync"
	"time"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/adaptor"

	"github.com/rfyiamcool/go-timewheel"

	"github.com/samber/mo"
)

// ttlEntry is a cache entry with a TTL.
type ttlEntry struct {
	ino        Ino
	dirtyPage  DirtyPage
	forever    bool
	expireTime time.Time
	task       *timewheel.Task
}

// DirtyPageCache is a cache for file data.
type DirtyPageCache struct {
	mu       sync.RWMutex
	ino2data map[Ino]*ttlEntry
	storage  adaptor.StorageBackend
	tw       *timewheel.TimeWheel
	opt      *option
}

type option struct {
	defaultTTL time.Duration
}

func NewDirtyPageCache(storage adaptor.StorageBackend) *DirtyPageCache {
	tw, err := timewheel.NewTimeWheel(1*time.Second, 360)
	if err != nil {
		panic(err)
	}

	tw.Start()
	return &DirtyPageCache{
		ino2data: make(map[Ino]*ttlEntry),
		storage:  storage,
		tw:       tw,
		opt:      &option{defaultTTL: time.Minute * 1},
	}
}

func (c *DirtyPageCache) GetBuffer(in Ino) mo.Option[DirtyPage] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.ino2data[in]
	if !ok || (!e.forever && e.expireTime.Before(time.Now())) {
		return mo.None[DirtyPage]()
	}

	return mo.Some(e.dirtyPage)

}
func (c *DirtyPageCache) Delete(in Ino) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.ino2data[in]
	if ok {
		if entry.dirtyPage.Idle(time.Minute) {
			c.tw.Remove(entry.task)
			delete(c.ino2data, in)
			entry.dirtyPage.Destroy()
		}
	}
}

func (c *DirtyPageCache) Flush(ctx context.Context, in Ino, page DirtyPage) error {
	c.mu.Lock()
	if ttl, found := c.ino2data[in]; found {
		c.tw.Remove(ttl.task)
	}

	c.ino2data[in] = &ttlEntry{
		ino:        in,
		dirtyPage:  page,
		expireTime: time.Now().Add(c.opt.defaultTTL),
		task: c.tw.AddCron(c.opt.defaultTTL, func() {
			c.Delete(in)
		}),
	}
	c.mu.Unlock()

	return page.Flush()
}
