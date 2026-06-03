package helpers

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// FreePort allocates an OS-assigned TCP port and returns it as a string.
// The listener is closed before return; the port is *probably* free for the
// caller to bind, with the usual TOCTOU caveat — fine for tests, not for prod.
func FreePort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}
