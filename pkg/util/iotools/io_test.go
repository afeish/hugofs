package iotools

import (
	"testing"

	"github.com/pingcap/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRand(t *testing.T) {
	log.SetLevel(zap.DebugLevel)
	fname, _, err := RandFile("hello", 10240)
	require.Nil(t, err)
	log.Info("file created", zap.String("name", fname))

	files := []string{"a", "b"}
	for _, f := range files {
		log.Info("file", zap.String("name", f))
	}
}
