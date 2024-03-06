//go:build !excludeTest

package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrite(t *testing.T) {
	file, err := os.OpenFile("/tmp/hugo/1", os.O_RDWR|os.O_CREATE, 0755)
	require.NoError(t, err)

	n, err := file.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.NoError(t, file.Close())
}
