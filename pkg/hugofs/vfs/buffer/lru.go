package buffer

import (
	"container/list"
	"sync"

	"github.com/afeish/hugo/pkg/hugofs/vfs/meta"
	lru "github.com/hashicorp/golang-lru/v2"
)

type BlockLRU struct {
	sync.RWMutex
	lru       *lru.Cache[meta.BlockKey, bool]
	inoKeyMap map[uint64]*list.List // key is ino, value is list of block that is used
	counter   int
	cap       int
}

func NewBlockLRU(size int, onEvit func(key meta.BlockKey, value bool)) (*BlockLRU, error) {
	cache, err := lru.NewWithEvict(size, onEvit)
	if err != nil {
		return nil, err
	}
	return &BlockLRU{
		lru:       cache,
		inoKeyMap: make(map[uint64]*list.List),
		cap:       size,
	}, nil
}

func (c *BlockLRU) Get(key meta.BlockKey) (bool, bool) {
	return c.lru.Get(key)
}

func (c *BlockLRU) Put(key meta.BlockKey) {
	c.Lock()
	defer c.Unlock()

	l, ok := c.inoKeyMap[key.GetIno()]
	if !ok {
		l = list.New()
		c.inoKeyMap[key.GetIno()] = l
	}
	l.PushBack(key)
	c.lru.Add(key, true)
	c.counter++
}

func (c *BlockLRU) RemoveOldest() (k meta.BlockKey, ok bool) {
	c.Lock()
	defer c.Unlock()
	k, _, ok = c.lru.RemoveOldest()
	l, ok := c.inoKeyMap[k.GetIno()]
	if ok {
		for i := l.Front(); i != nil; i = i.Next() {
			if i.Value.(meta.BlockKey) == k {
				l.Remove(i)
				c.counter--
				return
			}
		}
	}
	return
}

func (c *BlockLRU) RemoveByIno(ino uint64) (n int) {
	c.Lock()
	defer c.Unlock()
	l, ok := c.inoKeyMap[ino]
	if ok {
		var next *list.Element
		for i := l.Front(); i != nil; i = next {
			c.lru.Remove(i.Value.(meta.BlockKey))
			next = i.Next() //https://stackoverflow.com/questions/27662614/how-to-remove-element-from-list-while-iterating-the-same-list-in-golang
			l.Remove(i)
			n++
			c.counter--
		}
		delete(c.inoKeyMap, ino)
	}

	return
}

func (c *BlockLRU) Size() int {
	c.RLock()
	defer c.RUnlock()
	return c.counter
}

func (c *BlockLRU) Cap() int {
	return c.cap
}
