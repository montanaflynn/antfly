package db

import (
	"context"
	"errors"
	"testing"

	"github.com/antflydb/antfly/lib/schema"
	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/src/store/db/indexes"
	"github.com/antflydb/antfly/src/store/searchwire"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
	blevequery "github.com/blevesearch/bleve/v2/search/query"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSearchWireFuzzyFastPath(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"body": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for _, doc := range []struct {
		id   string
		body string
	}{
		{id: "doc-1", body: "hello world"},
		{id: "doc-2", body: "goodbye world"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextFuzzyRequest("full_text_index", "body", "helo", 0, 1, false, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextFuzzy)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireMatchAllAndMatchNoneFastPath(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"body": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	payload, err := json.Marshal(map[string]any{"body": "hello world"})
	require.NoError(t, err)
	err = db.Batch(ctx, [][2][]byte{{[]byte("doc-1"), payload}}, nil, Op_SyncLevelFullText)
	if err != nil && !errors.Is(err, ErrPartialSuccess) {
		require.NoError(t, err)
	}

	matchAllBytes := encodeSearchWireTextMatchAllRequest("full_text_index", 10, 0)
	matchAllRes, err := db.Search(ctx, matchAllBytes)
	require.NoError(t, err)
	matchAllTotal, matchAllHits, err := searchwire.DecodeHits(matchAllRes, searchWireOpTextMatchAll)
	require.NoError(t, err)
	require.Equal(t, uint64(1), matchAllTotal)
	require.Len(t, matchAllHits, 1)
	require.Equal(t, "doc-1", matchAllHits[0].ID)

	matchNoneBytes := encodeSearchWireTextMatchNoneRequest("full_text_index", 10, 0)
	matchNoneRes, err := db.Search(ctx, matchNoneBytes)
	require.NoError(t, err)
	matchNoneTotal, matchNoneHits, err := searchwire.DecodeHits(matchNoneRes, searchWireOpTextMatchNone)
	require.NoError(t, err)
	require.Equal(t, uint64(0), matchNoneTotal)
	require.Len(t, matchNoneHits, 0)
}

func TestSearchWireDateRangeStringFastPath(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"published_at": map[string]any{"type": "string", "format": "date-time", "x-antfly-types": []any{"datetime"}},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for key, doc := range map[string]map[string]any{
		"doc-1": {"published_at": "2025-01-01T00:00:00Z"},
		"doc-2": {"published_at": "2025-01-02T00:00:00Z"},
		"doc-3": {"published_at": "2025-01-03T00:00:00Z"},
	} {
		payload, err := json.Marshal(doc)
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(key), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	orig := blevequery.QueryDateTimeParser
	blevequery.QueryDateTimeParser = "dateTimeOptional"
	defer func() { blevequery.QueryDateTimeParser = orig }()

	reqBytes := encodeSearchWireTextDateRangeRequest("full_text_index", "published_at", "2025-01-01", "2025-01-02", nil, nil, "", 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextDateRange)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireNumericRangeFastPath(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"price": map[string]any{"type": "number"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for key, doc := range map[string]map[string]any{
		"doc-1": {"price": 10.0},
		"doc-2": {"price": 20.0},
		"doc-3": {"price": 30.0},
	} {
		payload, err := json.Marshal(doc)
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(key), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	min := 15.0
	max := 30.0
	reqBytes := encodeSearchWireTextNumericRangeRequest("full_text_index", "price", &min, &max, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextNumericRange)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-2", hits[0].ID)
}

func TestSearchWireGeoFastPaths(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string", "x-antfly-types": []any{"geopoint"}},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for key, doc := range map[string]map[string]any{
		"doc-1": {"location": map[string]any{"lat": 37.7749, "lon": -122.4194}},
		"doc-2": {"location": map[string]any{"lat": 37.7750, "lon": -122.4195}},
		"doc-3": {"location": map[string]any{"lat": 40.7128, "lon": -74.0060}},
	} {
		payload, err := json.Marshal(doc)
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(key), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	distanceBytes := encodeSearchWireTextGeoDistanceRequest("full_text_index", "location", -122.4194, 37.7749, "2km", 10, 0)
	distanceRes, err := db.Search(ctx, distanceBytes)
	require.NoError(t, err)
	distanceTotal, distanceHits, err := searchwire.DecodeHits(distanceRes, searchWireOpTextGeoDistance)
	require.NoError(t, err)
	require.Equal(t, uint64(2), distanceTotal)
	require.Len(t, distanceHits, 2)

	boxBytes := encodeSearchWireTextGeoBoundingBoxRequest("full_text_index", "location", -122.6, 37.9, -122.2, 37.7, 10, 0)
	boxRes, err := db.Search(ctx, boxBytes)
	require.NoError(t, err)
	boxTotal, boxHits, err := searchwire.DecodeHits(boxRes, searchWireOpTextGeoBBox)
	require.NoError(t, err)
	require.Equal(t, uint64(2), boxTotal)
	require.Len(t, boxHits, 2)

	polygonBytes := encodeSearchWireTextGeoBoundingPolygonRequest("full_text_index", "location", []blevegeo.Point{
		{Lon: -122.6, Lat: 37.9},
		{Lon: -122.2, Lat: 37.9},
		{Lon: -122.2, Lat: 37.7},
		{Lon: -122.6, Lat: 37.7},
	}, 10, 0)
	polygonRes, err := db.Search(ctx, polygonBytes)
	require.NoError(t, err)
	polygonTotal, polygonHits, err := searchwire.DecodeHits(polygonRes, searchWireOpTextGeoPolygon)
	require.NoError(t, err)
	require.Equal(t, uint64(2), polygonTotal)
	require.Len(t, polygonHits, 2)
}

func TestSearchWireTermRangeFastPath(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for key, doc := range map[string]map[string]any{
		"doc-1": {"title": "alpha"},
		"doc-2": {"title": "beta"},
		"doc-3": {"title": "gamma"},
	} {
		payload, err := json.Marshal(doc)
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(key), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	inclusiveMin := true
	inclusiveMax := false
	reqBytes := encodeSearchWireTextTermRangeRequest("full_text_index", "title", "beta", "gamma", &inclusiveMin, &inclusiveMax, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextTermRange)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-2", hits[0].ID)
}

func TestSearchWireDocIDBoolFieldAndIPRangeFastPaths(t *testing.T) {
	dir := t.TempDir()
	db := &DBImpl{logger: zaptest.NewLogger(t)}
	require.NoError(t, db.Open(dir, false, nil, types.Range{nil, []byte{0xFF}}))
	defer db.Close()

	tableSchema := &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{"type": "string"},
						"active":  map[string]any{"type": "boolean"},
						"ip":      map[string]any{"type": "string", "x-antfly-types": []any{"ip"}},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for key, doc := range map[string]map[string]any{
		"doc-1": {"content": "alpha beta", "active": true, "ip": "192.168.1.10"},
		"doc-2": {"content": "beta gamma", "active": false, "ip": "192.168.1.99"},
		"doc-3": {"content": "gamma delta", "active": true, "ip": "10.0.0.5"},
	} {
		payload, err := json.Marshal(doc)
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(key), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	docIDBytes := encodeSearchWireTextDocIDRequest([]string{"doc-1", "doc-3"}, 10, 0)
	docIDRes, err := db.Search(ctx, docIDBytes)
	require.NoError(t, err)
	docIDTotal, docIDHits, err := searchwire.DecodeHits(docIDRes, searchWireOpTextDocID)
	require.NoError(t, err)
	require.Equal(t, uint64(2), docIDTotal)
	require.Len(t, docIDHits, 2)

	boolBytes := encodeSearchWireTextBoolFieldRequest("full_text_index", "active", true, 10, 0)
	boolRes, err := db.Search(ctx, boolBytes)
	require.NoError(t, err)
	boolTotal, boolHits, err := searchwire.DecodeHits(boolRes, searchWireOpTextBoolField)
	require.NoError(t, err)
	require.Equal(t, uint64(2), boolTotal)
	require.Len(t, boolHits, 2)

	ipBytes := encodeSearchWireTextIPRangeRequest("full_text_index", "ip", "192.168.1.0/24", 10, 0)
	ipRes, err := db.Search(ctx, ipBytes)
	require.NoError(t, err)
	ipTotal, ipHits, err := searchwire.DecodeHits(ipRes, searchWireOpTextIPRange)
	require.NoError(t, err)
	require.Equal(t, uint64(2), ipTotal)
	require.Len(t, ipHits, 2)
}
