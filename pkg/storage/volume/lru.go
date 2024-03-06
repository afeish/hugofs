package volume

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

// BlockLRU track the active block in the physical volume
type BlockLRU struct {
	backed *lru.Cache[int, string] // K is blockID, V is sha256 of the block (used as key on s3)

	cap int
}

func NewBlockLRU(size int, onEvit func(key int, value string)) (*BlockLRU, error) {
	cache, err := lru.NewWithEvict(size, onEvit)
	if err != nil {
		return nil, err
	}
	return &BlockLRU{
		backed: cache,
		cap:    size,
	}, nil
}

func (c *BlockLRU) Get(key int) (string, bool) {
	return c.backed.Get(key)
}

func (c *BlockLRU) Put(key int, value string) {
	c.backed.Add(key, value)
}

func (c *BlockLRU) RemoveOldest() (key int, value string, ok bool) {
	return c.backed.RemoveOldest()
}

func (c *BlockLRU) Delete(key int) bool {
	return c.backed.Remove(key)
}

func (c *BlockLRU) Size() int {
	return c.backed.Len()
}

func (c *BlockLRU) Cap() int {
	return c.cap
}

func (c *BlockLRU) Keys() []int {
	return c.backed.Keys()
}

func (c *BlockLRU) Contains(key int) bool {
	return c.backed.Contains(key)
}

func (c *BlockLRU) Clear() {
	c.backed.Purge()
}

func (c *BlockLRU) Warmup() error {
	return nil
}
