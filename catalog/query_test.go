package catalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCollectionFromItems(t *testing.T) {
	item := map[string]interface{}{
		"type":         "Feature",
		"stac_version": "1.1.0",
		"id":           "item-one",
		"bbox":         []interface{}{16.1, 48.1, 16.6, 48.4},
		"properties":   map[string]interface{}{"datetime": "2026-05-19T10:00:22Z"},
		"links": []map[string]interface{}{{
			"rel": "self", "href": "https://package.example/catalog/share/item-one/",
		}},
	}
	collection := CollectionFromItems(
		"share",
		"https://package.example/catalog/share",
		map[string]interface{}{
			"type":         "FeatureCollection",
			"stac_version": "1.1.0",
			"features":     []map[string]interface{}{item},
		},
	)

	encoded, err := json.Marshal(collection)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Extent struct {
			Spatial struct {
				BBox [][]float64 `json:"bbox"`
			} `json:"spatial"`
			Temporal struct {
				Interval [][]interface{} `json:"interval"`
			} `json:"temporal"`
		} `json:"extent"`
		Links []map[string]interface{} `json:"links"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "Collection" || decoded.ID != "share" {
		t.Fatalf("unexpected STAC Collection: %s", encoded)
	}
	if !reflect.DeepEqual(decoded.Extent.Spatial.BBox, [][]float64{{16.1, 48.1, 16.6, 48.4}}) {
		t.Fatalf("unexpected spatial extent: %#v", decoded.Extent.Spatial.BBox)
	}
	if len(decoded.Links) != 2 || decoded.Links[1]["rel"] != "item" {
		t.Fatalf("expected self and item links, got %#v", decoded.Links)
	}
}

func TestCollectionFromItemsAcceptsDatabaseTimes(t *testing.T) {
	collection := CollectionFromItems("share", "https://package.example/catalog/share", map[string]interface{}{
		"type": "Feature",
		"id":   "item-one",
		"properties": map[string]interface{}{
			"datetime": time.Date(2026, time.February, 18, 10, 7, 11, 0, time.UTC),
		},
	})
	extent := collection["extent"].(map[string]interface{})
	temporal := extent["temporal"].(map[string]interface{})["interval"].([][]interface{})
	want := "2026-02-18T10:07:11Z"
	if temporal[0][0] != want || temporal[0][1] != want {
		t.Fatalf("expected temporal extent %q, got %#v", want, temporal)
	}
}

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

func TestRewriteAssetHrefs(t *testing.T) {
	tests := []struct {
		name      string
		href      string
		sharePath string
		mappings  []AssetMapping
		want      string
	}{
		{
			name:      "relative path",
			href:      "packages/item/data.tif",
			sharePath: "/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/packages/item/data.tif?presign&followRedirect",
		},
		{
			name:      "relative path with source query",
			href:      "packages/item/data.tif?source=one",
			sharePath: "/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/packages/item/data.tif?presign&followRedirect",
		},
		{
			name:      "absolute HTTPS path inside share",
			href:      "https://objects.example/my-bucket/deliverables/26-06/item/data.tif",
			sharePath: "/my-bucket/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/item/data.tif?presign&followRedirect",
		},
		{
			name:      "absolute HTTPS path with bucket before share",
			href:      "https://objects.example/my-bucket/deliverables/26-06/item/data.tif",
			sharePath: "/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/item/data.tif?presign&followRedirect",
		},
		{
			name:      "root-relative path does not duplicate share",
			href:      "/deliverables/26-06/item/data.tif",
			sharePath: "/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/item/data.tif?presign&followRedirect",
		},
		{
			name:      "S3 URL inside service-root share",
			href:      "s3://my-bucket/deliverables/26-06/item/data.tif",
			sharePath: "/my-bucket/deliverables/26-06",
			want:      "https://package.example/api/public/share/my-share/item/data.tif?presign&followRedirect",
		},
		{
			name:      "S3 URL at service root",
			href:      "s3://my-bucket/deliverables/26-06/item/data.tif",
			sharePath: "/",
			want:      "https://package.example/api/public/share/my-share/my-bucket/deliverables/26-06/item/data.tif?presign&followRedirect",
		},
		{
			name:      "external HTTPS URL at service root",
			href:      "https://external.example/other/data.tif",
			sharePath: "/",
			want:      "https://external.example/other/data.tif",
		},
		{
			name:      "external absolute URL",
			href:      "https://external.example/other/data.tif",
			sharePath: "/my-bucket/deliverables/26-06",
			want:      "https://external.example/other/data.tif",
		},
		{
			name:      "invalid empty mapping is ignored",
			href:      "https://external.example/other/data.tif",
			sharePath: "/deliverables/26-06",
			mappings:  []AssetMapping{{From: "", To: "packages"}},
			want:      "https://external.example/other/data.tif",
		},
		{
			name:      "root-relative path outside share",
			href:      "/other/data.tif",
			sharePath: "/deliverables/26-06",
			want:      "/other/data.tif",
		},
		{
			name:      "relative path cannot leave share",
			href:      "../private/data.tif",
			sharePath: "/deliverables/26-06",
			want:      "../private/data.tif",
		},
		{
			name:      "explicit mapping for OpenAerialMap URL",
			href:      "https://imagery.example.org/openaerialmap/67793f0b9478720001790586/thumbnail.png",
			sharePath: "/deliverables/26-06",
			mappings:  []AssetMapping{{From: "https://imagery.example.org/openaerialmap/", To: "openaerialmap-assets"}},
			want:      "https://package.example/api/public/share/my-share/openaerialmap-assets/67793f0b9478720001790586/thumbnail.png?presign&followRedirect",
		},
		{
			name:      "explicit S3 mapping to share root",
			href:      "s3://data/item.tif",
			sharePath: "/deliverables/26-06",
			mappings:  []AssetMapping{{From: "s3://data/", To: "."}},
			want:      "https://package.example/api/public/share/my-share/item.tif?presign&followRedirect",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := map[string]interface{}{
				"assets": map[string]interface{}{
					"data": map[string]interface{}{"href": test.href},
				},
			}
			rewriteAssetHrefs(entry, "https://package.example/api/public/share/my-share", test.sharePath, test.mappings)
			got := entry["assets"].(map[string]interface{})["data"].(map[string]interface{})["href"]
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("expected %q, got %#v", test.want, got)
			}
		})
	}
}

func TestHasMatchingAssetChecksAllAssets(t *testing.T) {
	entry := map[string]interface{}{
		"assets": map[string]interface{}{
			"external": map[string]interface{}{"href": "https://external.example/data.tif"},
			"package":  map[string]interface{}{"href": "packages/one/data.tif"},
		},
	}

	if !hasMatchingAsset(entry, "/deliverables/26-06", "packages/one", nil) {
		t.Fatal("expected package path to match one of the catalog assets")
	}
	if hasMatchingAsset(entry, "/deliverables/26-06", "packages/two", nil) {
		t.Fatal("did not expect an unrelated package path to match")
	}
}
