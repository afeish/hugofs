//go:build !excludeTest

package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/txnkv"
)

func TestTiKVClient(t *testing.T) {
	client, err := txnkv.NewClient([]string{"localhost:2379"})
	require.Nil(t, err)

	txn, err := client.Begin()
	require.Nil(t, err)

	key := "@fOo1"
	value := "bar"
	err = txn.Set([]byte(key), []byte(value))
	require.Nil(t, err)

	get, err := txn.Get(context.Background(), []byte(key))
	require.Nil(t, err)
	require.Equal(t, value, string(get))
}
