package testenv

import (
	"net"
	"strings"
	"testing"
)

// RequireLoopbackBind skips the test when the current environment cannot bind a
// local loopback listener, which some integration-style tests require.
func RequireLoopbackBind(t *testing.T) {
	t.Helper()

	candidates := []struct {
		network string
		address string
	}{
		{network: "tcp6", address: "[::1]:0"},
		{network: "tcp4", address: "127.0.0.1:0"},
	}

	var failures []string
	for _, candidate := range candidates {
		listener, err := net.Listen(candidate.network, candidate.address)
		if err == nil {
			_ = listener.Close()
			return
		}
		failures = append(failures, candidate.network+" "+candidate.address+": "+err.Error())
	}

	t.Skipf("loopback listener unavailable in this environment: %s", strings.Join(failures, "; "))
}
