package mem

import (
	"testing"

	"github.com/pingcap/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocateFree(t *testing.T) {
	buf := Allocate(12)
	Free(buf)
	if cap(buf) != min_size {
		t.Errorf("min size error allocated capacity=%d", cap(buf))
	}
	if len(buf) != 12 {
		t.Errorf("size error")
	}

	buf = Allocate(4883)
	Free(buf)
	if cap(buf) != 1024<<bitCount(4883) {
		t.Errorf("min size error allocated capacity=%d", cap(buf))
	}
	if len(buf) != 4883 {
		t.Errorf("size error")
	}

}

func TestAllocateTwice(t *testing.T) {
	buf1 := Allocate(11)
	log.S().Infof("%s\n", string(buf1))
	copy(buf1, []byte("hello world"))
	require.EqualValues(t, 1024, AllocMemory())
	Free(buf1)

	buf2 := Allocate(11)
	log.S().Infof("%s\n", string(buf2))
	copy(buf2, []byte("dlrow olleh"))
	require.EqualValues(t, 1024, AllocMemory())
	Free(buf2)

	buf3 := Allocate(13)
	log.S().Infof("%s\n", string(buf3))
	copy(buf3, []byte("dlrow olleh!"))
	require.EqualValues(t, 1024, AllocMemory())

	buf4 := Allocate(13)
	log.S().Infof("%s\n", string(buf4))
	copy(buf4, []byte("dlrow olleh!"))
	require.EqualValues(t, 2048, AllocMemory())
}

func TestAllocateFreeEdgeCases(t *testing.T) {
	assert.Equal(t, 1, bitCount(2048))
	assert.Equal(t, 2, bitCount(2049))

	buf := Allocate(2048)
	Free(buf)
	buf = Allocate(2049)
	Free(buf)
}

func TestBitCount(t *testing.T) {
	count := bitCount(12)
	if count != 0 {
		t.Errorf("bitCount error count=%d", count)
	}
	if count != bitCount(min_size) {
		t.Errorf("bitCount error count=%d", count)
	}

}
