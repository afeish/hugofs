package volume

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestEvitBlockJson(t *testing.T) {
	tuples := []lo.Tuple2[int, string]{
		{A: 1, B: "a"},
		{A: 2, B: "b"},
		{A: 3, B: "c"},
		{A: 4, B: "d"},
		{A: 5, B: "e"},
		{A: 6, B: "f"},
	}
	j, err := json.Marshal(tuples)
	require.Nil(t, err)

	tuples2 := make([]lo.Tuple2[int, string], 0)
	err = json.Unmarshal(j, &tuples2)
	require.Nil(t, err)

	require.EqualValues(t, tuples, tuples2)
}
