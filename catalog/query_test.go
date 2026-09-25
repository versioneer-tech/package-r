package catalog

import (
	"strings"
	"testing"
)

func TestRedactCatalogURL(t *testing.T) {
	const signedURL = "https://objects.example.invalid/catalog.parquet?signature=my-secret"
	message := redactCatalogURL("failed to read "+signedURL, signedURL)

	if strings.Contains(message, "my-secret") {
		t.Fatalf("signed URL was not redacted: %q", message)
	}
	if !strings.Contains(message, "[signed catalog URL]") {
		t.Fatalf("redaction marker is missing: %q", message)
	}
}
