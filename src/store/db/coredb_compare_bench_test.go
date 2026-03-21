//go:build zigdb

package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/antflydb/antfly/lib/schema"
	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/lib/vector"
	json "github.com/antflydb/antfly/pkg/libaf/json"
	"github.com/antflydb/antfly/src/store/db/indexes"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.uber.org/zap"
)

type benchmarkBackend struct {
	name string
	open func(*testing.B, *schema.TableSchema) DB
}

type benchmarkSearchSize struct {
	name  string
	count int
}

type benchmarkVectorCase struct {
	name  string
	count int
	dim   int
}

func benchmarkSchema() *schema.TableSchema {
	return &schema.TableSchema{
		DefaultType: "default",
		DocumentSchemas: map[string]schema.DocumentSchema{
			"default": {
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title":    map[string]any{"type": "string"},
						"content":  map[string]any{"type": "string"},
						"category": map[string]any{"type": "string"},
						"score":    map[string]any{"type": "number"},
						"embedding": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "number"},
						},
					},
				},
			},
		},
	}
}

func benchmarkBackends() []benchmarkBackend {
	return []benchmarkBackend{
		{
			name: "go",
			open: func(b *testing.B, tableSchema *schema.TableSchema) DB {
				b.Helper()
				db := NewDBImpl(zap.NewNop(), nil, tableSchema, map[string]indexes.IndexConfig{}, nil, nil, nil)
				requireOpenBenchmarkDB(b, db, tableSchema)
				return db
			},
		},
		{
			name: "zig",
			open: func(b *testing.B, tableSchema *schema.TableSchema) DB {
				b.Helper()
				db := NewZigCoreDB(zap.NewNop(), nil, tableSchema, map[string]indexes.IndexConfig{}, nil, nil, nil)
				requireOpenBenchmarkDB(b, db, tableSchema)
				return db
			},
		},
	}
}

func requireOpenBenchmarkDB(b *testing.B, db DB, tableSchema *schema.TableSchema) {
	b.Helper()
	if err := db.Open(b.TempDir(), false, tableSchema, types.Range{nil, []byte{0xFF}}); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Fatal(err)
		}
	})
}

func benchmarkDocJSON(i int) []byte {
	doc := map[string]any{
		"title":     fmt.Sprintf("document %06d", i),
		"content":   fmt.Sprintf("alpha benchmark token %06d", i),
		"category":  fmt.Sprintf("cat-%02d", i%8),
		"score":     float64(i % 100),
		"embedding": []float32{1, float32(i % 10)},
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return buf
}

func benchmarkVectorValues(i, dim int) []float32 {
	values := make([]float32, dim)
	for j := range values {
		values[j] = float32(((i + j) % 17)) / 17
	}
	if dim > 0 {
		values[0] = 1
	}
	return values
}

func benchmarkVectorDocJSON(i int, dim int) []byte {
	doc := map[string]any{
		"title":     fmt.Sprintf("vector %06d", i),
		"content":   fmt.Sprintf("vector benchmark token %06d", i),
		"category":  fmt.Sprintf("vec-%02d", i%8),
		"score":     float64(i % 100),
		"embedding": benchmarkVectorValues(i, dim),
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return buf
}

func benchmarkSearchSizes() []benchmarkSearchSize {
	return []benchmarkSearchSize{
		{name: "docs=2048", count: 2048},
		{name: "docs=16384", count: 16384},
	}
}

func benchmarkVectorCases() []benchmarkVectorCase {
	return []benchmarkVectorCase{
		{name: "docs=2048/dim=2", count: 2048, dim: 2},
		{name: "docs=8192/dim=128", count: 8192, dim: 128},
	}
}

func benchmarkKey(i int) []byte {
	return []byte(fmt.Sprintf("doc-%06d", i))
}

func requireSeedDocs(b *testing.B, db DB, count int, syncLevel Op_SyncLevel, encode func(int) []byte) {
	b.Helper()
	ctx := context.Background()
	const batchSize = 128
	for start := 0; start < count; start += batchSize {
		end := start + batchSize
		if end > count {
			end = count
		}
		writes := make([][2][]byte, 0, end-start)
		for i := start; i < end; i++ {
			writes = append(writes, [2][]byte{benchmarkKey(i), encode(i)})
		}
		if err := requireBenchmarkBatch(ctx, db, writes, syncLevel); err != nil {
			b.Fatal(err)
		}
	}
}

func requireBenchmarkBatch(ctx context.Context, db DB, writes [][2][]byte, syncLevel Op_SyncLevel) error {
	err := db.Batch(ctx, writes, nil, syncLevel)
	if err != nil && !errors.Is(err, ErrPartialSuccess) {
		return err
	}
	waitForBenchmarkIndexes(ctx, db)
	return nil
}

func requireBenchmarkEnqueue(ctx context.Context, db DB, writes [][2][]byte, syncLevel Op_SyncLevel) error {
	err := db.Batch(ctx, writes, nil, syncLevel)
	if err != nil && !errors.Is(err, ErrPartialSuccess) {
		return err
	}
	return nil
}

func waitForBenchmarkIndexes(ctx context.Context, db DB) {
	switch typed := db.(type) {
	case *DBImpl:
		typed.indexManager.WaitForBackfills(ctx)
	}
}

func requireAddIndex(b *testing.B, db DB, cfg indexes.IndexConfig) {
	b.Helper()
	if err := db.AddIndex(cfg); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkCoreDBBackends(b *testing.B) {
	tableSchema := benchmarkSchema()

	for _, backend := range benchmarkBackends() {
		backend := backend

		b.Run(backend.name+"/Write", func(b *testing.B) {
			db := backend.open(b, tableSchema)
			ctx := context.Background()
			value := benchmarkDocJSON(0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := db.Batch(ctx, [][2][]byte{{benchmarkKey(i), value}}, nil, Op_SyncLevelWrite); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(backend.name+"/Lookup", func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireSeedDocs(b, db, 1024, Op_SyncLevelWrite, benchmarkDocJSON)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Get(ctx, benchmarkKey(i%1024)); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(backend.name+"/Scan", func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireSeedDocs(b, db, 2048, Op_SyncLevelWrite, benchmarkDocJSON)
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Scan(ctx, benchmarkKey(256), benchmarkKey(1536), ScanOptions{IncludeDocuments: false}); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(backend.name+"/FullTextWriteEnqueue", func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
			ctx := context.Background()
			value := benchmarkDocJSON(0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := requireBenchmarkEnqueue(ctx, db, [][2][]byte{{benchmarkKey(i), value}}, Op_SyncLevelFullText); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(backend.name+"/FullTextWriteVisible", func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
			ctx := context.Background()
			value := benchmarkDocJSON(0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := requireBenchmarkBatch(ctx, db, [][2][]byte{{benchmarkKey(i), value}}, Op_SyncLevelFullText); err != nil {
					b.Fatal(err)
				}
			}
		})

		for _, searchSize := range benchmarkSearchSizes() {
			searchSize := searchSize
			b.Run(backend.name+"/FullTextSearch/"+searchSize.name, func(b *testing.B) {
				db := backend.open(b, tableSchema)
				requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
				requireSeedDocs(b, db, searchSize.count, Op_SyncLevelFullText, benchmarkDocJSON)

				matchAlpha := query.NewMatchQuery("alpha")
				matchAlpha.SetField("content")
				req := &indexes.RemoteIndexSearchRequest{
					BleveSearchRequest: bleve.NewSearchRequest(matchAlpha),
					Limit:              10,
				}
				req.BleveSearchRequest.Size = 10
				var (
					reqBytes []byte
					err      error
				)
				if backend.name == "zig" {
					reqBytes = encodeSearchWireTextMatchRequest("full_text_index", "content", "alpha", "", 0, 0, false, 0, 10, 0)
				} else {
					reqBytes, err = json.Marshal(req)
					if err != nil {
						b.Fatal(err)
					}
				}

				ctx := context.Background()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := db.Search(ctx, reqBytes); err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		for _, vectorCase := range benchmarkVectorCases() {
			vectorCase := vectorCase
			b.Run(backend.name+"/DenseVectorSearch/"+vectorCase.name, func(b *testing.B) {
				db := backend.open(b, tableSchema)
				requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
				requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
					Field:          "embedding",
					Dimension:      vectorCase.dim,
					DistanceMetric: indexes.DistanceMetricL2Squared,
				}))
				requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
					return benchmarkVectorDocJSON(i, vectorCase.dim)
				})

				req := &indexes.RemoteIndexSearchRequest{
					VectorSearches: map[string]vector.T{
						"dense_idx": benchmarkVectorValues(0, vectorCase.dim),
					},
					Limit: 10,
				}
				var (
					reqBytes []byte
					err      error
				)
				if backend.name == "zig" {
					reqBytes = encodeSearchWireDenseRequest("dense_idx", benchmarkVectorValues(0, vectorCase.dim), 10, 10, 0)
				} else {
					reqBytes, err = json.Marshal(req)
					if err != nil {
						b.Fatal(err)
					}
				}

				ctx := context.Background()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := db.Search(ctx, reqBytes); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
