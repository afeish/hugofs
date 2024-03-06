package volume

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlotCoding(t *testing.T) {
	hashPairs := make(map[int]string, 0)

	data, err := json.Marshal(hashPairs)
	require.Nil(t, err)
	require.Len(t, data, 2)

	hashPairs2 := make(map[int]string, 0)
	err = json.Unmarshal(data, &hashPairs2)
	require.Nil(t, err)
	require.Len(t, hashPairs2, 0)

	hashPairs[1] = "test"
	hashPairs[2] = "ok"
	data, err = json.Marshal(hashPairs)
	require.Nil(t, err)

	hashPairs2 = make(map[int]string, 0)
	err = json.Unmarshal(data, &hashPairs2)
	require.Nil(t, err)
	require.Len(t, hashPairs2, 2)
}

func TestHash(t *testing.T) {
	hash := "8da238fd42e271d2efffb103f919db1b931ae61f946a78f1e4b3d8a7d26b5cbd"

	bytes, err := hex.DecodeString(hash)
	require.Nil(t, err)
	fmt.Println(bytes)
}
