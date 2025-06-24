package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

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

func rewriteAssetHrefs(entry map[string]interface{}, baseURL, presignedURL string) {
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
		if !ok || !strings.HasPrefix(href, baseURL) {
			continue
		}
		relativePath := strings.TrimPrefix(href, baseURL)
		newHref := strings.TrimRight(presignedURL, "/") + "/" + relativePath + "?presign&followRedirect"
		asset["href"] = newHref
	}
}

func QueryCatalogParquet(ctx context.Context, catalogPath, filterField, baseURL, requestPath, assetsURL string) (map[string]interface{}, error) {
	query := fmt.Sprintf(`
SELECT *
FROM read_parquet('%s')
WHERE COALESCE((CAST(assets AS JSON)->'$.%s'->>'href'), '') LIKE '%s%%%%'
`, catalogPath, filterField, baseURL+requestPath)

	log.Printf("Query: %s", query)

	conn, err := GetDuckDBConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
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

		if _, ok := entry["type"]; !ok {
			entry["type"] = "Feature"
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
				delete(entry, "geometry")
			}
		}

		if _, ok := entry["properties"]; !ok {
			entry["properties"] = map[string]interface{}{}
		}

		if repo, ok := entry["repository"]; ok {
			if props, ok := entry["properties"].(map[string]interface{}); ok {
				props["repository"] = repo
			}
			delete(entry, "repository")
		}

		if _, ok := entry["links"]; ok {
			delete(entry, "links")
		}

		delete(entry, "href")

		rewriteAssetHrefs(entry, baseURL, assetsURL)

		results = append(results, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("Total STAC features: %d", len(results))

	if len(results) == 1 {
		return results[0], nil
	}
	return map[string]interface{}{
		"type":     "FeatureCollection",
		"features": results,
	}, nil
}
