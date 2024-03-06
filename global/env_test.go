package global

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCfg(t *testing.T) {
	t.Setenv("HUGO_LOG_LEVEL", "error")
	t.Setenv("HUGO_LOG_FILE_ENABLED", "true")
	t.Setenv("HUGO_LOG_DIR", "/var/log/hugo")
	t.Setenv("PYROSCOPE_URL", "http://172.18.118.207:4040")
	t.Setenv("HUGO_TRANSPORT", "tcp")

	cfg := GetEnvCfg()
	require.EqualValues(t, "error", cfg.Log.Level)
	require.EqualValues(t, true, cfg.Log.FileEnabled)
	require.EqualValues(t, "/var/log/hugo", cfg.Log.Dir)
	require.EqualValues(t, "http://172.18.118.207:4040", cfg.Tracing.PyroscopeURL)
	require.EqualValues(t, "tcp", cfg.Mount.Transport)

	t.Setenv("HUGO_LOG_LEVEL", "panic")
	ReloadEnvCfg()
	cfg = GetEnvCfg()
	require.EqualValues(t, "panic", cfg.Log.Level)

}
