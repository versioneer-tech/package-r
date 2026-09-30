package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	appLogging "github.com/versioneer-tech/package-r/logging"
)

const defaultSTACVersion = "1.1.0"

func tryParseJSON(val interface{}) interface{} {
	s, ok := val.(string)
	if !ok || len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return val
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return val
}

func bboxToPolygon(bbox []float64) map[string]interface{} {
	xmin, ymin, xmax, ymax := bbox[0], bbox[1], bbox[2], bbox[3]
	return map[string]interface{}{
		"type": "Polygon",
		"coordinates": [][][]float64{{
			{xmin, ymin},
			{xmax, ymin},
			{xmax, ymax},
			{xmin, ymax},
			{xmin, ymin},
		}},
	}
}

func isZeroBBox(bbox []float64) bool {
	return len(bbox) == 4 && bbox[0] == 0 && bbox[1] == 0 && bbox[2] == 0 && bbox[3] == 0
}

func isRelativeAssetHref(href string) bool {
	if href == "" || strings.HasPrefix(href, "/") {
		return false
	}
	parsed, err := url.Parse(href)
	return err == nil && !parsed.IsAbs()
}

// AssetMapping maps a nonstandard catalog href prefix to a path inside a share.
type AssetMapping struct {
	From string
	To   string
}

// QueryOptions describes one catalog request.
type QueryOptions struct {
	CatalogURL      string
	RequestPath     string
	AssetsURL       string
	CatalogEndpoint string
	SharePath       string
	AssetMappings   []AssetMapping
}

func relativeAssetPrefix(sharePath, requestPath string) string {
	cleanSharePath := path.Clean("/" + strings.TrimPrefix(sharePath, "/"))
	cleanRequestPath := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	if cleanRequestPath == "/" {
		return ""
	}

	if cleanSharePath == "/" {
		return strings.TrimPrefix(cleanRequestPath, "/")
	}

	if cleanRequestPath == cleanSharePath {
		return ""
	}
	if strings.HasPrefix(cleanRequestPath, cleanSharePath+"/") {
		return strings.TrimPrefix(cleanRequestPath, cleanSharePath+"/")
	}

	shareEntryPath := "/" + path.Base(cleanSharePath)
	if cleanRequestPath == shareEntryPath {
		return ""
	}
	if strings.HasPrefix(cleanRequestPath, shareEntryPath+"/") {
		return strings.TrimPrefix(cleanRequestPath, shareEntryPath+"/")
	}

	return strings.TrimPrefix(cleanRequestPath, "/")
}

func cleanRelativeAssetPath(value string) (string, bool) {
	if strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return "", false
	}
	clean := path.Clean(value)
	if clean == "." {
		return "", true
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func pathInShare(candidate, sharePath string, allowEmbeddedSharePath bool) (string, bool) {
	cleanCandidate := path.Clean("/" + strings.TrimPrefix(candidate, "/"))
	cleanShare := path.Clean("/" + strings.TrimPrefix(sharePath, "/"))
	if cleanShare == "/" {
		return strings.TrimPrefix(cleanCandidate, "/"), true
	}

	prefix := strings.TrimRight(cleanShare, "/") + "/"
	if strings.HasPrefix(cleanCandidate, prefix) {
		return strings.TrimPrefix(cleanCandidate, prefix), true
	}
	if allowEmbeddedSharePath {
		if index := strings.Index(cleanCandidate, prefix); index >= 0 {
			return strings.TrimPrefix(cleanCandidate[index:], prefix), true
		}
	}
	return "", false
}

func absoluteAssetPathInShare(href, sharePath string) (string, bool) {
	parsed, err := url.Parse(href)
	if err != nil || parsed.Host == "" {
		return "", false
	}

	var candidates []string
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if path.Clean("/"+strings.TrimPrefix(sharePath, "/")) == "/" {
			return "", false
		}
		candidates = []string{parsed.Path}
	case "s3":
		candidates = []string{"/" + parsed.Host + parsed.Path, parsed.Path}
	default:
		return "", false
	}

	for _, candidate := range candidates {
		if relative, ok := pathInShare(candidate, sharePath, true); ok {
			return relative, true
		}
	}
	return "", false
}

func mappedAssetPath(href string, mappings []AssetMapping) (string, bool) {
	for _, mapping := range mappings {
		if mapping.From == "" || !strings.HasPrefix(href, mapping.From) {
			continue
		}
		suffix := strings.TrimLeft(strings.TrimPrefix(href, mapping.From), "/")
		if before, _, found := strings.Cut(suffix, "?"); found {
			suffix = before
		}
		if before, _, found := strings.Cut(suffix, "#"); found {
			suffix = before
		}
		return cleanRelativeAssetPath(path.Join(mapping.To, suffix))
	}
	return "", false
}

func assetPathInShare(href, sharePath string, mappings []AssetMapping) (string, bool) {
	if relative, ok := mappedAssetPath(href, mappings); ok {
		return relative, true
	}
	if isRelativeAssetHref(href) {
		parsed, _ := url.Parse(href)
		return cleanRelativeAssetPath(parsed.Path)
	}
	if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
		parsed, err := url.Parse(href)
		if err != nil {
			return "", false
		}
		return pathInShare(parsed.Path, sharePath, true)
	}
	return absoluteAssetPathInShare(href, sharePath)
}

func rewriteAssetHrefs(entry map[string]interface{}, assetsURL, sharePath string, mappings []AssetMapping) {
	assetsRaw, ok := entry["assets"]
	if !ok {
		return
	}
	assets, ok := assetsRaw.(map[string]interface{})
	if !ok {
		return
	}

	for _, v := range assets {
		asset, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		href, ok := asset["href"].(string)
		if !ok {
			continue
		}

		relativePath, internal := assetPathInShare(href, sharePath, mappings)
		if !internal || relativePath == "" {
			continue
		}

		newHref := strings.TrimRight(assetsURL, "/") + "/" + relativePath + "?presign&followRedirect"
		asset["href"] = newHref
	}
}

func hasMatchingAsset(entry map[string]interface{}, sharePath, relativePrefix string, mappings []AssetMapping) bool {
	assetsRaw, ok := entry["assets"]
	if !ok {
		return false
	}
	assets, ok := assetsRaw.(map[string]interface{})
	if !ok {
		return false
	}

	for _, v := range assets {
		asset, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		href, ok := asset["href"].(string)
		if !ok {
			continue
		}
		if relativePrefix == "" {
			return true
		}
		relativePath, internal := assetPathInShare(href, sharePath, mappings)
		if internal && (relativePath == relativePrefix || strings.HasPrefix(relativePath, strings.TrimRight(relativePrefix, "/")+"/")) {
			return true
		}
	}
	return false
}

func normalizeItemProperties(entry map[string]interface{}) {
	properties, ok := entry["properties"].(map[string]interface{})
	if !ok {
		properties = map[string]interface{}{}
		entry["properties"] = properties
	}

	itemFields := map[string]bool{
		"type":            true,
		"stac_version":    true,
		"stac_extensions": true,
		"id":              true,
		"geometry":        true,
		"bbox":            true,
		"properties":      true,
		"links":           true,
		"assets":          true,
		"href":            true,
	}
	for key, value := range entry {
		if itemFields[key] {
			continue
		}
		if _, exists := properties[key]; !exists {
			properties[key] = value
		}
		delete(entry, key)
	}

	if properties["datetime"] == nil && properties["start_datetime"] == nil && properties["end_datetime"] == nil {
		if created := properties["created"]; created != nil {
			properties["datetime"] = created
		}
	}
}

func itemSelfHref(entry map[string]interface{}, assetsURL, catalogEndpoint string) string {
	assets, ok := entry["assets"].(map[string]interface{})
	if !ok {
		return ""
	}
	assetPrefix := strings.TrimRight(assetsURL, "/") + "/"
	keys := make([]string, 0, len(assets))
	for key := range assets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := assets[key]
		asset, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		href, ok := asset["href"].(string)
		if !ok || !strings.HasPrefix(href, assetPrefix) {
			continue
		}
		relativePath := strings.TrimPrefix(href, assetPrefix)
		if before, _, found := strings.Cut(relativePath, "?"); found {
			relativePath = before
		}
		if relativePath != "" {
			itemPath := path.Dir(relativePath)
			if itemPath == "." {
				itemPath = relativePath
			} else {
				itemPath += "/"
			}
			return strings.TrimRight(catalogEndpoint, "/") + "/" + itemPath
		}
	}
	return ""
}

// CollectionFromItems creates a static STAC Collection that links to the Items
// returned by a catalog query.
func CollectionFromItems(id, selfHref string, result map[string]interface{}) map[string]interface{} {
	items := collectionItems(result)
	links := []map[string]interface{}{
		{
			"rel":  "self",
			"href": selfHref,
			"type": "application/json",
		},
	}

	for _, item := range items {
		itemID, _ := item["id"].(string)
		for _, link := range itemLinks(item) {
			if link["rel"] != "self" || link["href"] == "" {
				continue
			}
			links = append(links, map[string]interface{}{
				"rel":   "item",
				"href":  link["href"],
				"type":  "application/geo+json",
				"title": itemID,
			})
			break
		}
	}

	spatial, temporal := collectionExtent(items)
	return map[string]interface{}{
		"type":         "Collection",
		"stac_version": collectionSTACVersion(result, items),
		"id":           id,
		"title":        id,
		"description":  "STAC catalog for the " + id + " public share.",
		"license":      "various",
		"extent": map[string]interface{}{
			"spatial":  map[string]interface{}{"bbox": [][]float64{spatial}},
			"temporal": map[string]interface{}{"interval": [][]interface{}{{temporal[0], temporal[1]}}},
		},
		"links": links,
	}
}

func collectionItems(result map[string]interface{}) []map[string]interface{} {
	if result["type"] == "Feature" {
		return []map[string]interface{}{result}
	}
	items, _ := result["features"].([]map[string]interface{})
	if items != nil {
		return items
	}
	rawItems, _ := result["features"].([]interface{})
	items = make([]map[string]interface{}, 0, len(rawItems))
	for _, raw := range rawItems {
		if item, ok := raw.(map[string]interface{}); ok {
			items = append(items, item)
		}
	}
	return items
}

func itemLinks(item map[string]interface{}) []map[string]interface{} {
	links, _ := item["links"].([]map[string]interface{})
	if links != nil {
		return links
	}
	rawLinks, _ := item["links"].([]interface{})
	links = make([]map[string]interface{}, 0, len(rawLinks))
	for _, raw := range rawLinks {
		if link, ok := raw.(map[string]interface{}); ok {
			links = append(links, link)
		}
	}
	return links
}

func collectionSTACVersion(result map[string]interface{}, items []map[string]interface{}) string {
	if version, ok := result["stac_version"].(string); ok && version != "" {
		return version
	}
	for _, item := range items {
		if version, ok := item["stac_version"].(string); ok && version != "" {
			return version
		}
	}
	return defaultSTACVersion
}

func collectionExtent(items []map[string]interface{}) ([]float64, []interface{}) {
	spatial := []float64{-180, -90, 180, 90}
	var minTime, maxTime time.Time
	hasSpatial := false

	for _, item := range items {
		if bbox, ok := numericBBox(item["bbox"]); ok {
			if !hasSpatial {
				spatial = bbox
				hasSpatial = true
			} else {
				spatial[0] = min(spatial[0], bbox[0])
				spatial[1] = min(spatial[1], bbox[1])
				spatial[2] = max(spatial[2], bbox[2])
				spatial[3] = max(spatial[3], bbox[3])
			}
		}

		properties, _ := item["properties"].(map[string]interface{})
		for _, key := range []string{"datetime", "start_datetime", "end_datetime"} {
			parsed, ok := catalogTime(properties[key])
			if !ok {
				continue
			}
			if minTime.IsZero() || parsed.Before(minTime) {
				minTime = parsed
			}
			if maxTime.IsZero() || parsed.After(maxTime) {
				maxTime = parsed
			}
		}
	}

	temporal := []interface{}{nil, nil}
	if !minTime.IsZero() {
		temporal[0] = minTime.Format(time.RFC3339Nano)
	}
	if !maxTime.IsZero() {
		temporal[1] = maxTime.Format(time.RFC3339Nano)
	}
	return spatial, temporal
}

func catalogTime(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		return parsed, err == nil
	default:
		return time.Time{}, false
	}
}

func numericBBox(value interface{}) ([]float64, bool) {
	if bbox, ok := value.([]float64); ok && len(bbox) == 4 {
		return append([]float64(nil), bbox...), true
	}
	raw, ok := value.([]interface{})
	if !ok || len(raw) != 4 {
		return nil, false
	}
	bbox := make([]float64, 4)
	for index, coordinate := range raw {
		number, ok := coordinate.(float64)
		if !ok {
			return nil, false
		}
		bbox[index] = number
	}
	return bbox, true
}

//nolint:gocyclo
func QueryCatalogParquet(ctx context.Context, options QueryOptions) (result map[string]interface{}, err error) {
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		appLogging.Timedf(
			elapsed,
			"duckdb catalog-query request-path=%q duration=%s error=%v",
			options.RequestPath,
			elapsed,
			err,
		)
	}()
	relativePrefix := relativeAssetPrefix(options.SharePath, options.RequestPath)

	// Asset layouts vary between supported catalog schemas, so the query reads
	// all rows and the loop filters them after parsing the asset metadata.
	query := `
SELECT *
FROM read_parquet(?)
`
	args := []interface{}{options.CatalogURL}

	conn, err := GetDuckDBConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	queryStarted := time.Now()
	rows, err := conn.QueryContext(ctx, query, args...)
	queryElapsed := time.Since(queryStarted)
	if err != nil {
		message := redactCatalogURL(err.Error(), options.CatalogURL)
		queryErr := errors.New("query failed: " + message)
		appLogging.Timedf(
			queryElapsed,
			"duckdb execute-query duration=%s error=%v",
			queryElapsed,
			queryErr,
		)
		return nil, queryErr
	}
	appLogging.Timedf(
		queryElapsed,
		"duckdb execute-query duration=%s error=<nil>",
		queryElapsed,
	)
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading columns failed: %w", err)
	}

	results := make([]map[string]interface{}, 0, 100)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		entry := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			entry[col] = tryParseJSON(values[i])
		}

		if !hasMatchingAsset(entry, options.SharePath, relativePrefix, options.AssetMappings) {
			continue
		}

		if _, ok := entry["type"]; !ok {
			entry["type"] = "Feature"
		}
		if _, ok := entry["stac_version"]; !ok {
			entry["stac_version"] = defaultSTACVersion
		}

		if bbox, ok := entry["bbox"].(map[string]interface{}); ok {
			xmin, xminOk := bbox["xmin"].(float64)
			ymin, yminOk := bbox["ymin"].(float64)
			xmax, xmaxOk := bbox["xmax"].(float64)
			ymax, ymaxOk := bbox["ymax"].(float64)
			if xminOk && yminOk && xmaxOk && ymaxOk {
				entry["bbox"] = []float64{xmin, ymin, xmax, ymax}
			}
		}

		if bbox, ok := entry["bbox"].([]float64); ok && isZeroBBox(bbox) {
			delete(entry, "bbox")
		}

		if geomStr, ok := entry["geometry"].(string); ok && (len(geomStr) > 0 && geomStr[0] == '{') {
			var geomObj map[string]interface{}
			if err := json.Unmarshal([]byte(geomStr), &geomObj); err == nil {
				entry["geometry"] = geomObj
			}
		}

		geom, hasGeom := entry["geometry"].(map[string]interface{})
		replaceGeom := false

		if hasGeom {
			coords, ok := geom["coordinates"].([]interface{})
			if ok && len(coords) > 0 {
				if poly, ok := coords[0].([]interface{}); ok {
					allZero := true
					for _, pt := range poly {
						if pair, ok := pt.([]interface{}); ok && len(pair) == 2 {
							if pair[0] != float64(0) || pair[1] != float64(0) {
								allZero = false
								break
							}
						}
					}
					replaceGeom = allZero
				}
			} else {
				replaceGeom = true
			}
		} else {
			replaceGeom = true
		}

		if replaceGeom {
			if bbox, ok := entry["bbox"].([]float64); ok && len(bbox) == 4 {
				entry["geometry"] = bboxToPolygon(bbox)
			} else {
				entry["geometry"] = nil
			}
		}

		normalizeItemProperties(entry)

		if repo, ok := entry["repository"]; ok {
			if props, ok := entry["properties"].(map[string]interface{}); ok {
				props["repository"] = repo
			}
			delete(entry, "repository")
		}

		entry["links"] = []interface{}{}

		delete(entry, "href")

		rewriteAssetHrefs(entry, options.AssetsURL, options.SharePath, options.AssetMappings)

		if selfHref := itemSelfHref(entry, options.AssetsURL, options.CatalogEndpoint); selfHref != "" {
			entry["links"] = []map[string]interface{}{
				{
					"rel":  "self",
					"href": selfHref,
					"type": "application/geo+json",
				},
			}
		}

		results = append(results, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	appLogging.Debugf("duckdb catalog-query features=%d", len(results))

	if len(results) == 1 {
		return results[0], nil
	}
	stacVersion := defaultSTACVersion
	for _, result := range results {
		if version, ok := result["stac_version"].(string); ok && version != "" {
			stacVersion = version
			break
		}
	}
	return map[string]interface{}{
		"type":         "FeatureCollection",
		"stac_version": stacVersion,
		"links": []map[string]interface{}{
			{
				"rel":  "self",
				"href": strings.TrimRight(options.CatalogEndpoint, "/"),
				"type": "application/geo+json",
			},
		},
		"features": results,
	}, nil
}

func redactCatalogURL(message, catalogURL string) string {
	if catalogURL == "" {
		return message
	}
	return strings.ReplaceAll(message, catalogURL, "[signed catalog URL]")
}
