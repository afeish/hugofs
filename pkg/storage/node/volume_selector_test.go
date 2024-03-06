package node

import (
	"sort"
	"testing"

	vol "github.com/afeish/hugo/pkg/storage/volume"
	"github.com/stretchr/testify/require"
)

func TestSelector(t *testing.T) {
	volStats := []vol.VolumeStatistics{
		{Quota: 1024, InuseBytes: 100, VolID: 1},
		{Quota: 1024, InuseBytes: 200, VolID: 2},
	}
	sort.SliceStable(volStats, func(i, j int) bool {
		return volStats[i].RemainCap() > volStats[j].RemainCap()
	})
	require.EqualValues(t, 1, volStats[0].VolID)

	volStats[0].InuseBytes = 300
	sort.SliceStable(volStats, func(i, j int) bool {
		return volStats[i].RemainCap() > volStats[j].RemainCap()
	})
	require.EqualValues(t, 2, volStats[0].VolID)
}
