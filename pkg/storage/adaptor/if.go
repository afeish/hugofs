package adaptor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/afeish/hugo/global"
	meta "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pb/storage"
	mapset "github.com/deckarep/golang-set/v2"
)

type Healthy interface {
	IsHealthy(ctx context.Context) bool
}

type StorageClient interface {
	storage.StorageServiceClient
	storage.VolumeServiceClient
	Healthy
	String() string
	global.Closable
}

type MetaServiceClient interface {
	meta.EventServiceClient
	meta.RawNodeServiceClient
	meta.StorageMetaClient
	meta.ClusterServiceClient
	meta.StorageMeta_HeartBeatClient
	Healthy
	global.Closable
}

var (
	keyLocker = keyLockCache{cache: make(map[BlockKey]*sync.RWMutex)}
)

type keyLockCache struct {
	sync.RWMutex
	cache map[BlockKey]*sync.RWMutex
}

func (c *keyLockCache) LockFor(key BlockKey) {
	c.Lock()
	mu, found := c.cache[key]
	if !found {
		mu = &sync.RWMutex{}
		c.cache[key] = mu
		c.Unlock()
		mu.Lock()
		return
	}
	c.Unlock()
	mu.Lock()
}

func (c *keyLockCache) RLockFor(key BlockKey) {
	c.Lock()
	mu, found := c.cache[key]
	if !found {
		mu = &sync.RWMutex{}
		c.cache[key] = mu
		c.Unlock()
		mu.RLock()
		return
	}
	c.Unlock()
	mu.RLock()
}

func (c *keyLockCache) UnlockFor(key BlockKey) {
	c.RLock()
	mu, found := c.cache[key]
	if !found {
		c.RUnlock()
		return
	}
	c.RUnlock()
	mu.Unlock()
}

func (c *keyLockCache) RUnlockFor(key BlockKey) {
	c.RLock()
	mu, found := c.cache[key]
	if !found {
		c.RUnlock()
		return
	}
	c.RUnlock()
	mu.RUnlock()
}

func (c *keyLockCache) Delete(key BlockKey) {
	c.Lock()
	defer c.Unlock()
	delete(c.cache, key)
}

func RemoveBlockKey(set mapset.Set[BlockKey]) {
	for elem := range set.Iter() {
		keyLocker.Delete(elem)
	}
}

var (
	EmptyBlockKey = NewBlockKey(0, 0)
)

type BlockKey string

func NewBlockKey(ino uint64, index uint64) BlockKey {
	return BlockKey(fmt.Sprintf("%d,%d", ino, index))
}

func (k BlockKey) Lock() {
	keyLocker.LockFor(k)
}

func (k BlockKey) Unlock() {
	keyLocker.UnlockFor(k)
}

func (k BlockKey) RLock() {
	keyLocker.RLockFor(k)
}

func (k BlockKey) RUnlock() {
	keyLocker.RUnlockFor(k)
}

func (k BlockKey) Split() (ino uint64, index uint64) {
	inoStr, indexStr, _ := strings.Cut(string(k), ",")
	ino, _ = strconv.ParseUint(inoStr, 10, 64)
	index, _ = strconv.ParseUint(indexStr, 10, 64)
	return
}

func (k BlockKey) GetIno() uint64 {
	inoStr, _, _ := strings.Cut(string(k), ",")
	ino, _ := strconv.ParseUint(inoStr, 10, 64)
	return ino
}

func (k BlockKey) GetIndex() uint64 {
	_, indexStr, _ := strings.Cut(string(k), ",")
	index, _ := strconv.ParseUint(indexStr, 10, 64)
	return index
}

func (k BlockKey) String() string {
	return string(k)
}
