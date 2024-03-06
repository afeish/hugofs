package gateway

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAddr(t *testing.T) {
	host, port, err := net.SplitHostPort("127.0.0.1:8080")
	require.NoError(t, err)
	t.Log(host)
	t.Log(port)

	p, err := net.LookupPort("", port)
	require.NoError(t, err)
	t.Log(p)
}
