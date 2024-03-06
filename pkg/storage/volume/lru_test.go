package volume

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLru(t *testing.T) {
	var (
		lru, _ = NewBlockLRU(3, func(key int, value string) {
			fmt.Printf("%d of [ %s ] is evicted\n", key, value)
		})
	)

	lru.Put(1, "a1")
	lru.Put(2, "a2")
	lru.Put(3, "a3")
	lru.Put(4, "a4")

	require.Equal(t, 3, lru.Size())
	require.False(t, lru.Contains(1))
	require.True(t, lru.Contains(2))
}
