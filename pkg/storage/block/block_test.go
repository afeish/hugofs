package block

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializeBlockHeader(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	block, _ := NewBlock(1, data)
	length, _ := block.GetLen()
	require.EqualValues(t, len(data), length)
}
