package cmd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthcheckCommand(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requested atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested.Store(true)
				assert.Equal(t, "/health", r.URL.Path)
				w.Header().Set("Location", "/")
				w.WriteHeader(status)
			}))
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			require.NoError(t, err)
			output, err := runPackageRCommandResult(t, "healthcheck", "--address", host, "--port", port)
			require.True(t, requested.Load(), "health endpoint was not requested: %s", output)
			if status == http.StatusOK {
				require.NoError(t, err, "%s", output)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestHealthcheckWildcardAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	for _, address := range []string{"", "0.0.0.0", "::", "[::]"} {
		require.NoError(t, checkHealth(t.Context(), address, port))
	}
}

func TestHealthcheckCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, checkHealth(ctx, "127.0.0.1", "8888"), context.Canceled)
}
