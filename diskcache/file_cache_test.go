package diskcache

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestFileCache(t *testing.T) {
	ctx := context.Background()
	cache := New(afero.NewMemMapFs(), "/cache")
	for _, value := range []string{"some text", "new text"} {
		require.NoError(t, cache.Store(ctx, "key", []byte(value)))
		got, exists, err := cache.Load(ctx, "key")
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, value, string(got))
	}
	require.NoError(t, cache.Delete(ctx, "key"))
	_, exists, err := cache.Load(ctx, "key")
	require.NoError(t, err)
	require.False(t, exists)
}
