package files

import (
	"mime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtensionTypesAreValid(t *testing.T) {
	for extension, mediaType := range extensionTypes {
		t.Run(extension, func(t *testing.T) {
			parsed, _, err := mime.ParseMediaType(mediaType)
			require.NoError(t, err)
			require.Equal(t, mediaType, parsed)
			require.NotEmpty(t, extension)
			require.Equal(t, byte('.'), extension[0])
		})
	}
}

func TestRegisteredExtensionTypes(t *testing.T) {
	tests := map[string]string{
		".gz":       "application/gzip",
		".markdown": "text/markdown",
		".md":       "text/markdown",
		".mid":      "audio/midi",
		".opus":     "audio/ogg",
		".otf":      "font/otf",
		".parquet":  "application/vnd.apache.parquet",
		".py":       "text/x-python",
		".rpm":      "application/x-rpm",
		".tif":      "image/tiff",
		".wasm":     "application/wasm",
		".yaml":     "application/yaml",
	}

	for extension, expected := range tests {
		t.Run(extension, func(t *testing.T) {
			actual, _, err := mime.ParseMediaType(mime.TypeByExtension(extension))
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
}
