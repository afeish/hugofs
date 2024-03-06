package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

type B struct {
	UnimplementedSerializer
	ID int
}

func (b *B) FormatKey() string {
	return fmt.Sprintf("b/%d", b.ID)
}
func (b *B) FormatPrefix() string {
	return "b"
}
func (b *B) Serialize() ([]byte, error) {
	x, err := json.Marshal(b)
	return x, err
}
func (b *B) Deserialize(bytes []byte) (*B, error) {
	var x B
	if err := json.Unmarshal(bytes, &x); err != nil {
		return nil, err
	}
	*b = x
	return &x, nil
}
func (b *B) Self() *B {
	return b
}

func TestTxnRollback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := GetStore(BadgerName, mo.Some("/tmp/badger"))
	require.NoError(t, err)

	t.Cleanup(func() {
		s.Cleanup(ctx)
	})

	var expectedErr error = errors.New("expected error")

	var set = []struct {
		id        string
		aval      A
		bval      B
		throwErr  bool
		err       error
		expectLen int
	}{
		{
			id:        "1",
			aval:      A{ID: 1},
			bval:      B{ID: 2},
			throwErr:  true,
			err:       expectedErr,
			expectLen: 0,
		},
		{
			id:        "2",
			aval:      A{ID: 1},
			bval:      B{ID: 2},
			throwErr:  false,
			err:       nil,
			expectLen: 1,
		},
	}

	for _, v := range set {
		t.Run(v.id, func(t *testing.T) {
			err = DoInTxn(ctx, s, func(ctx context.Context, txn Transaction) (err error) {
				err = SaveInKVPair(ctx, s, func() (Serializer[A], error) {
					return &v.aval, nil
				})
				if err != nil {
					return err
				}

				err = SaveInKVPair(ctx, s, func() (Serializer[B], error) {
					return &v.bval, nil
				})
				if err != nil {
					return err
				}
				if v.throwErr {
					return expectedErr
				}
				return nil
			})
			require.Equal(t, err, v.err)

			res, err := GetValuesByPrefix(ctx, s, func() (Serializer[B], error) {
				return &v.bval, nil
			})
			require.NoError(t, err)
			require.Equal(t, v.expectLen, len(res))
		})
	}

}
