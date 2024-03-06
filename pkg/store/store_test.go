package store

import (
	"context"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

func TestOpenMem(t *testing.T) {
	store, err := GetStore(MemoryName, mo.None[string]())
	require.NoError(t, err)

	err = store.Save(context.Background(), "a", []byte("b"))
	require.NoError(t, err)
	val, err := store.Get(context.Background(), "a")
	require.NoError(t, err)
	require.EqualValues(t, []byte("b"), val)

	getTxn, err := GetTxn(MemoryName, mo.Some[any](store))
	require.NoError(t, err)

	require.NoError(t, getTxn.Begin(context.Background()))
}
