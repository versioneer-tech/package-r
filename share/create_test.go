package share

import "testing"

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
