package db

import (
	"context"
	"errors"
	"testing"

	"github.com/antflydb/antfly/lib/schema"
	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/src/store/db/indexes"
	"github.com/antflydb/antfly/src/store/searchwire"
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
