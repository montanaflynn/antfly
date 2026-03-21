package db

import (
	"context"
	"errors"
	"testing"

	"github.com/antflydb/antfly/lib/schema"
	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/src/store/db/indexes"
	"github.com/antflydb/antfly/src/store/searchwire"
	"github.com/blevesearch/bleve/v2"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSearchWireBoolFastPath(t *testing.T) {
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
		{id: "doc-2", body: "hello exclude"},
		{id: "doc-3", body: "world only"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest(
		"full_text_index",
		[]searchWireTextClause{{Op: searchWireOpTextMatch, Field: "body", Text: "hello"}},
		nil,
		[]searchWireTextClause{{Op: searchWireOpTextMatch, Field: "body", Text: "exclude"}},
		10,
		0,
	)

	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireBoolFastPath_WithFuzzyClause(t *testing.T) {
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

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op:        searchWireOpTextFuzzy,
		Field:     "body",
		Text:      "helo",
		Fuzziness: 1,
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireBoolFastPath_PreservesMatchOperator(t *testing.T) {
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
		{id: "doc-1", body: "hello"},
		{id: "doc-2", body: "world"},
		{id: "doc-3", body: "hello world"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op:       searchWireOpTextMatch,
		Field:    "body",
		Text:     "hello world",
		Operator: uint8(query.MatchQueryOperatorAnd),
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-3", hits[0].ID)
}

func TestSearchWireBoolFastPath_PreservesMinShould(t *testing.T) {
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
		{id: "doc-1", body: "hello"},
		{id: "doc-2", body: "world"},
		{id: "doc-3", body: "hello world"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequestWithMin("full_text_index", nil, []searchWireTextClause{
		{Op: searchWireOpTextMatch, Field: "body", Text: "hello"},
		{Op: searchWireOpTextMatch, Field: "body", Text: "world"},
	}, nil, 2, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-3", hits[0].ID)
}

func TestSearchWireBoolFastPath_SupportsFilterClauses(t *testing.T) {
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
						"body":  map[string]any{"type": "string"},
						"title": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for _, doc := range []struct {
		id    string
		body  string
		title string
	}{
		{id: "doc-1", body: "hello world", title: "keep"},
		{id: "doc-2", body: "hello world", title: "drop"},
		{id: "doc-3", body: "goodbye world", title: "keep"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body, "title": doc.title})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequestWithFilter("full_text_index",
		[]searchWireTextClause{{Op: searchWireOpTextMatch, Field: "body", Text: "hello"}},
		nil,
		nil,
		[]searchWireTextClause{{Op: searchWireOpTextTerm, Field: "title", Text: "keep"}},
		0,
		10,
		0,
	)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireBoolFastPath_PreservesMatchPhraseAutoFuzziness(t *testing.T) {
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
		{id: "doc-1", body: "alpha beta"},
		{id: "doc-2", body: "alpha betx"},
		{id: "doc-3", body: "alpha gamma"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op:        searchWireOpTextMatchPhrase,
		Field:     "body",
		Text:      "alpha beta",
		Auto:      true,
		Fuzziness: 0,
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), total)
	require.Len(t, hits, 2)
}

func TestSearchWireBoolFastPath_PreservesPhraseAutoFuzziness(t *testing.T) {
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
		{id: "doc-1", body: "alpha beta"},
		{id: "doc-2", body: "alpha betx"},
		{id: "doc-3", body: "alpha gamma"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op:        searchWireOpTextPhrase,
		Field:     "body",
		Terms:     []string{"alpha", "beta"},
		Auto:      true,
		Fuzziness: 0,
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), total)
	require.Len(t, hits, 2)
}

func TestSearchWireBoolFastPath_PreservesMultiPhraseAutoFuzziness(t *testing.T) {
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
		{id: "doc-1", body: "alpha beta"},
		{id: "doc-2", body: "alpha betx"},
		{id: "doc-3", body: "alpha gamma"},
	} {
		payload, err := json.Marshal(map[string]any{"body": doc.body})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op:        searchWireOpTextMultiPhrase,
		Field:     "body",
		TermSets:  [][]string{{"alpha"}, {"beta", "betx"}},
		Auto:      true,
		Fuzziness: 0,
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), total)
	require.Len(t, hits, 2)
}

func TestSearchWireBoolFastPath_SupportsRangeDocIDBoolAndIPClauses(t *testing.T) {
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
						"title":     map[string]any{"type": "string"},
						"published": map[string]any{"type": "boolean"},
						"ip":        map[string]any{"type": "string", "x-antfly-types": []any{"ip"}},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for _, doc := range []struct {
		id        string
		title     string
		published bool
		ip        string
	}{
		{id: "doc-1", title: "beta", published: true, ip: "10.0.0.1"},
		{id: "doc-2", title: "gamma", published: false, ip: "10.0.0.2"},
		{id: "doc-3", title: "delta", published: true, ip: "192.168.1.10"},
	} {
		payload, err := json.Marshal(map[string]any{
			"title":     doc.title,
			"published": doc.published,
			"ip":        doc.ip,
		})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{
		{
			Op:      searchWireOpTextTermRange,
			Field:   "title",
			Text:    "beta",
			AltText: "delta",
			InclMin: true,
			InclMax: false,
		},
		{
			Op:        searchWireOpTextBoolField,
			Field:     "published",
			BoolValue: true,
		},
		{
			Op:    searchWireOpTextIPRange,
			Field: "ip",
			Text:  "10.0.0.0/24",
		},
		{
			Op:    searchWireOpTextDocID,
			Terms: []string{"doc-1", "doc-3"},
		},
	}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, hits, 1)
	require.Equal(t, "doc-1", hits[0].ID)
}

func TestSearchWireBoolFastPath_SupportsNumericAndDateRangeClauses(t *testing.T) {
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
						"score":      map[string]any{"type": "number"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for _, doc := range []struct {
		id        string
		score     float64
		createdAt string
	}{
		{id: "doc-1", score: 1.5, createdAt: "2024-01-15T00:00:00Z"},
		{id: "doc-2", score: 3.0, createdAt: "2024-02-20T00:00:00Z"},
		{id: "doc-3", score: 4.5, createdAt: "2024-03-10T00:00:00Z"},
	} {
		payload, err := json.Marshal(map[string]any{
			"score":      doc.score,
			"created_at": doc.createdAt,
		})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{
		{
			Op:        searchWireOpTextNumericRange,
			Field:     "score",
			NumMin:    1.0,
			HasNumMin: true,
			NumMax:    4.0,
			HasNumMax: true,
			InclMin:   true,
			InclMax:   false,
		},
		{
			Op:      searchWireOpTextDateRange,
			Field:   "created_at",
			Text:    "2024-01-01T00:00:00Z",
			AltText: "2024-03-01T00:00:00Z",
			InclMin: true,
			InclMax: false,
		},
	}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)
	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), total)
	require.Len(t, hits, 2)
}

func TestSearchWireBoolFastPath_SupportsGeoClauses(t *testing.T) {
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

	cases := []struct {
		name   string
		clause searchWireTextClause
	}{
		{
			name: "geo_distance",
			clause: searchWireTextClause{
				Op:       searchWireOpTextGeoDistance,
				Field:    "location",
				Lon:      -122.4194,
				Lat:      37.7749,
				Distance: "2km",
			},
		},
		{
			name: "geo_bbox",
			clause: searchWireTextClause{
				Op:             searchWireOpTextGeoBBox,
				Field:          "location",
				TopLeftLon:     -122.6,
				TopLeftLat:     37.9,
				BottomRightLon: -122.2,
				BottomRightLat: 37.7,
			},
		},
		{
			name: "geo_polygon",
			clause: searchWireTextClause{
				Op:    searchWireOpTextGeoPolygon,
				Field: "location",
				Points: []blevegeo.Point{
					{Lon: -122.6, Lat: 37.9},
					{Lon: -122.2, Lat: 37.9},
					{Lon: -122.2, Lat: 37.7},
					{Lon: -122.6, Lat: 37.7},
				},
			},
		},
		{
			name: "geo_shape",
			clause: searchWireTextClause{
				Op:       searchWireOpTextGeoShape,
				Field:    "location",
				Relation: "intersects",
				ShapePolygons: [][]blevegeo.Point{{
					{Lon: -122.6, Lat: 37.9},
					{Lon: -122.2, Lat: 37.9},
					{Lon: -122.2, Lat: 37.7},
					{Lon: -122.6, Lat: 37.7},
					{Lon: -122.6, Lat: 37.9},
				}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{tc.clause}, nil, nil, 10, 0)
			resBytes, err := db.Search(ctx, reqBytes)
			require.NoError(t, err)
			total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
			require.NoError(t, err)
			require.Equal(t, uint64(2), total)
			require.Len(t, hits, 2)
		})
	}
}

func TestDBImpl_FullTextBooleanGeoShapeQuery(t *testing.T) {
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

	shapeQ, err := query.NewGeoShapeQuery([][][][]float64{
		{
			{
				{-122.6, 37.9},
				{-122.2, 37.9},
				{-122.2, 37.7},
				{-122.6, 37.7},
				{-122.6, 37.9},
			},
		},
	}, "polygon", "intersects")
	require.NoError(t, err)
	shapeQ.SetField("location")

	req := &indexes.RemoteIndexSearchRequest{
		BleveSearchRequest: bleve.NewSearchRequest(query.NewBooleanQuery([]query.Query{shapeQ}, nil, nil)),
		Limit:              10,
	}
	req.BleveSearchRequest.Size = 10

	reqBytes, err := json.Marshal(req)
	require.NoError(t, err)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)

	var res indexes.RemoteIndexSearchResult
	require.NoError(t, json.Unmarshal(resBytes, &res))
	require.NotNil(t, res.BleveSearchResult)
	require.Len(t, res.BleveSearchResult.Hits, 2)
}

func TestSearchWireBoolFastPath_SupportsNestedBoolClauses(t *testing.T) {
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
						"title":   map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, db.UpdateSchema(tableSchema))
	require.NoError(t, db.AddIndex(*indexes.NewFullTextIndexConfig("full_text_index_v0", false)))

	ctx := context.Background()
	for _, doc := range []struct {
		id      string
		content string
		title   string
	}{
		{id: "doc-1", content: "alpha", title: "small"},
		{id: "doc-2", content: "alpha", title: "other"},
		{id: "doc-3", content: "beta", title: "small"},
	} {
		payload, err := json.Marshal(map[string]any{"content": doc.content, "title": doc.title})
		require.NoError(t, err)
		err = db.Batch(ctx, [][2][]byte{{[]byte(doc.id), payload}}, nil, Op_SyncLevelFullText)
		if err != nil && !errors.Is(err, ErrPartialSuccess) {
			require.NoError(t, err)
		}
	}

	reqBytes := encodeSearchWireTextBoolRequest("full_text_index", []searchWireTextClause{{
		Op: searchWireOpTextBool,
		Must: []searchWireTextClause{{
			Op:    searchWireOpTextMatch,
			Field: "content",
			Text:  "alpha",
		}},
		Should: []searchWireTextClause{{
			Op:    searchWireOpTextPrefix,
			Field: "title",
			Text:  "sm",
		}},
	}}, nil, nil, 10, 0)
	resBytes, err := db.Search(ctx, reqBytes)
	require.NoError(t, err)

	total, hits, err := searchwire.DecodeHits(resBytes, searchWireOpTextBool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), total)
	require.Len(t, hits, 2)
}
