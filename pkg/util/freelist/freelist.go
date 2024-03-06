package freelist

import (
	"sort"
	"sync"
)

// borrowed from github.com/google/btree@v1.1.2/btree_generic.go

const (
	DefaultFreeListSize = 32
)

// FreeListG represents a free list of btree nodes. By default each
// BTree has its own FreeList, but multiple BTrees can share the same
// FreeList, in particular when they're created with Clone.
// Two Btrees using the same freelist are safe for concurrent write access.
type FreeListG[T any] struct {
	mu       sync.Mutex
	freelist items[T]
	creator  func() T

	size int
}

// NewFreeListG creates a new free list.
// size is the maximum size of the returned free list.
func NewFreeListG[T any](size int, creator func() T) *FreeListG[T] {
	return &FreeListG[T]{
		freelist: make(items[T], 0, size),
		size:     size,
		creator:  creator,
	}
}

func NewFreeListUnboundG[T any](creator func() T) *FreeListG[T] {
	return &FreeListG[T]{
		freelist: make(items[T], 0),
		creator:  creator,
	}
}

func (f *FreeListG[T]) New() (n T) {
	f.mu.Lock()
	index := len(f.freelist) - 1
	if index < 0 {
		f.mu.Unlock()
		return f.creator()
	}
	n = f.freelist.removeAt(index)
	f.mu.Unlock()
	return
}

func (f *FreeListG[T]) NewWith(creator func() T) (n T) {
	f.mu.Lock()
	index := len(f.freelist) - 1
	if index < 0 {
		f.mu.Unlock()
		return creator()
	}
	n = f.freelist.removeAt(index)
	f.mu.Unlock()
	return
}

func (f *FreeListG[T]) Free(n T) (out bool) {
	f.mu.Lock()
	if f.size > 0 {
		if len(f.freelist) < cap(f.freelist) {
			f.freelist.insertAt(len(f.freelist), n)
			out = true
		}
	} else {
		f.freelist.insertAt(len(f.freelist), n)
	}

	f.mu.Unlock()
	return
}

// items stores items in a node.
type items[T any] []T

// insertAt inserts a value into the given index, pushing all subsequent values
// forward.
func (s *items[T]) insertAt(index int, item T) {
	var zero T
	*s = append(*s, zero)
	if index < len(*s) {
		copy((*s)[index+1:], (*s)[index:])
	}
	(*s)[index] = item
}

// removeAt removes a value at a given index, pulling all subsequent values
// back.
func (s *items[T]) removeAt(index int) T {
	item := (*s)[index]
	copy((*s)[index:], (*s)[index+1:])
	var zero T
	(*s)[len(*s)-1] = zero
	*s = (*s)[:len(*s)-1]
	return item
}

// pop removes and returns the last element in the list.
func (s *items[T]) pop() (out T) {
	index := len(*s) - 1
	out = (*s)[index]
	var zero T
	(*s)[index] = zero
	*s = (*s)[:index]
	return
}

// truncate truncates this instance at index so that it contains only the
// first index items. index must be less than or equal to length.
func (s *items[T]) truncate(index int) {
	var toClear items[T]
	*s, toClear = (*s)[:index], (*s)[index:]
	var zero T
	for i := 0; i < len(toClear); i++ {
		toClear[i] = zero
	}
}

// find returns the index where the given item should be inserted into this
// list.  'found' is true if the item already exists in the list at the given
// index.
func (s items[T]) find(item T, less func(T, T) bool) (index int, found bool) {
	i := sort.Search(len(s), func(i int) bool {
		return less(item, s[i])
	})
	if i > 0 && !less(s[i-1], item) {
		return i - 1, true
	}
	return i, false
}
