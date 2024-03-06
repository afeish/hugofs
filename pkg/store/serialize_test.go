package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

type A struct {
	UnimplementedSerializer
	ID int
}

func (a *A) FormatKey() string {
	return fmt.Sprintf("a/%d", a.ID)
}
func (a *A) FormatPrefix() string {
	return "a"
}
func (a *A) Serialize() ([]byte, error) {
	x, err := json.Marshal(a)
	return x, err
}
func (a *A) Deserialize(bytes []byte) (*A, error) {
	var x A
	if err := json.Unmarshal(bytes, &x); err != nil {
		return nil, err
	}
	*a = x
	return &x, nil
}
func (a *A) Self() *A {
	return a
}
func TestStoreImplement(t *testing.T) {
	m, err := GetStore(MemoryName, mo.None[string]())
	require.Nil(t, err)
	x := &A{ID: 1}
	buf, err := x.Serialize()
	require.Nil(t, err)
	fmt.Println(string(buf))
	require.Nil(t, DoSaveInKVPair[A](context.Background(), m, &A{ID: 1}))
	v, err := DoGetValueByKey[A](context.Background(), m, &A{ID: 1})
	require.Nil(t, err)
	require.Equal(t, 1, v.ID)
}
