package db

import (
	"context"
	"errors"
	"testing"

	"github.com/antflydb/antfly/lib/schema"
	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/src/store/db/indexes"
	"github.com/antflydb/antfly/src/store/searchwire"
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
