package share

import (
	"testing"
	"time"
)

func TestCompactExpiration(t *testing.T) {
	now := time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC)
	tests := map[string]time.Time{
		"30d": now.AddDate(0, 0, 30),
		"10d": now.AddDate(0, 0, 10),
		"2m":  now.AddDate(0, 2, 0),
		"3H":  now.Add(3 * time.Hour),
		"1y":  now.AddDate(1, 0, 0),
		"2y":  now.AddDate(2, 0, 0),
	}

	for value, expected := range tests {
		t.Run(value, func(t *testing.T) {
			got, err := compactExpiration(value, now)
			if err != nil {
				t.Fatal(err)
			}
			if got != expected.Unix() {
				t.Fatalf("expected %d, got %d", expected.Unix(), got)
			}
		})
	}
}

func TestCompactExpirationEmptyMeansNoExpiration(t *testing.T) {
	got, err := compactExpiration("", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("expected no expiration, got %d", got)
	}
}

func TestCompactExpirationRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"0d", "-1d", "+1d", "30", "2M", "3h", "1Y", "days"} {
		t.Run(value, func(t *testing.T) {
			if _, err := compactExpiration(value, time.Now()); err == nil {
				t.Fatalf("expected %q to be rejected", value)
			}
		})
	}
}

func TestCatalogPathUsesShareFolder(t *testing.T) {
	got, err := CatalogPath("/a/b", "catalog.parquet")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/a/b/catalog.parquet" {
		t.Fatalf("expected catalog path below shared folder, got %q", got)
	}
}

func TestCatalogPathAllowsNestedRelativePath(t *testing.T) {
	got, err := CatalogPath("/a/b", "meta/catalog.parquet")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/a/b/meta/catalog.parquet" {
		t.Fatalf("expected nested catalog path below shared folder, got %q", got)
	}
}

func TestCatalogPathRejectsEscapes(t *testing.T) {
	for _, name := range []string{
		"/catalog.parquet",
		"../catalog.parquet",
		"meta/../../catalog.parquet",
		".",
		"meta/..",
		"catalog\x00.parquet",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := CatalogPath("/a/b", name); err == nil {
				t.Fatalf("expected error for %q, got path %q", name, got)
			}
		})
	}
}
