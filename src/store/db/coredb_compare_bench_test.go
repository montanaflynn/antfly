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
	"github.com/antflydb/antfly/lib/vectorindex"
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

func benchmarkDenseDocJSON(indexName string, i int, dim int) []byte {
	doc := map[string]any{
		"title":    fmt.Sprintf("vector %06d", i),
		"content":  fmt.Sprintf("vector benchmark token %06d", i),
		"category": fmt.Sprintf("vec-%02d", i%8),
		"score":    float64(i % 100),
		"_embeddings": map[string]any{
			indexName: benchmarkVectorValues(i, dim),
		},
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
					return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
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

			if backend.name == "zig" {
				b.Run(backend.name+"/DenseCallNoop/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					zigDB := db.(*ZigCoreDB)

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if err := zigDB.bridge.DenseNoop(); err != nil {
							b.Fatal(err)
						}
					}
				})

				b.Run(backend.name+"/DenseCallFixedPacked/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					zigDB := db.(*ZigCoreDB)

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := zigDB.bridge.DenseFixedPackedResult("dense_idx"); err != nil {
							b.Fatal(err)
						}
					}
				})

				b.Run(backend.name+"/DenseVectorSearchProfile/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
					requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
						Field:          "embedding",
						Dimension:      vectorCase.dim,
						DistanceMetric: indexes.DistanceMetricL2Squared,
					}))
					requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
						return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
					})

					zigDB := db.(*ZigCoreDB)
					vectorQuery := benchmarkVectorValues(0, vectorCase.dim)
					var totalNS uint64
					var lookupNS uint64
					var searchNS uint64
					var hitsNS uint64
					var fallbackNS uint64

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						profile, err := zigDB.bridge.SearchDenseProfile("dense_idx", vectorQuery, 10, 10, 0)
						if err != nil {
							b.Fatal(err)
						}
						totalNS += profile.TotalNS
						lookupNS += profile.IndexLookupNS
						searchNS += profile.SearchNS
						hitsNS += profile.HitsNS
						fallbackNS += profile.FallbackNS
					}
					if b.N > 0 {
						denom := float64(b.N)
						b.ReportMetric(float64(totalNS)/denom, "zig_total_ns/op")
						b.ReportMetric(float64(lookupNS)/denom, "zig_lookup_ns/op")
						b.ReportMetric(float64(searchNS)/denom, "zig_search_ns/op")
						b.ReportMetric(float64(hitsNS)/denom, "zig_hits_ns/op")
						b.ReportMetric(float64(fallbackNS)/denom, "zig_fallback_ns/op")
					}
				})

				b.Run(backend.name+"/DenseVectorSearchBridge/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
					requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
						Field:          "embedding",
						Dimension:      vectorCase.dim,
						DistanceMetric: indexes.DistanceMetricL2Squared,
					}))
					requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
						return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
					})

					zigDB := db.(*ZigCoreDB)
					reqBytes := encodeSearchWireDenseRequest("dense_idx", benchmarkVectorValues(0, vectorCase.dim), 10, 10, 0)

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := zigDB.bridge.SearchDenseWireRaw(reqBytes); err != nil {
							b.Fatal(err)
						}
					}
				})

				b.Run(backend.name+"/DenseVectorSearchBridgePacked/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
					requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
						Field:          "embedding",
						Dimension:      vectorCase.dim,
						DistanceMetric: indexes.DistanceMetricL2Squared,
					}))
					requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
						return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
					})

					zigDB := db.(*ZigCoreDB)
					vectorQuery := benchmarkVectorValues(0, vectorCase.dim)

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := zigDB.bridge.SearchDenseResult("dense_idx", vectorQuery, 10, 10, 0); err != nil {
							b.Fatal(err)
						}
					}
				})

				b.Run(backend.name+"/DenseVectorSearchFastPath/"+vectorCase.name, func(b *testing.B) {
					db := backend.open(b, tableSchema)
					requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
					requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
						Field:          "embedding",
						Dimension:      vectorCase.dim,
						DistanceMetric: indexes.DistanceMetricL2Squared,
					}))
					requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
						return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
					})

					zigDB := db.(*ZigCoreDB)
					reqBytes := encodeSearchWireDenseRequest("dense_idx", benchmarkVectorValues(0, vectorCase.dim), 10, 10, 0)
					ctx := context.Background()

					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := zigDB.searchWireFastPath(ctx, zigDB.bridge, reqBytes, searchWireOpDenseKnn); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func BenchmarkZigDenseVectorSearchProfile(b *testing.B) {
	tableSchema := benchmarkSchema()
	backend := benchmarkBackend{
		name: "zig",
		open: func(b *testing.B, tableSchema *schema.TableSchema) DB {
			b.Helper()
			db := NewZigCoreDB(zap.NewNop(), nil, tableSchema, map[string]indexes.IndexConfig{}, nil, nil, nil)
			requireOpenBenchmarkDB(b, db, tableSchema)
			return db
		},
	}

	for _, vectorCase := range benchmarkVectorCases() {
		vectorCase := vectorCase
		b.Run(vectorCase.name, func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
			requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
				Field:          "embedding",
				Dimension:      vectorCase.dim,
				DistanceMetric: indexes.DistanceMetricL2Squared,
			}))
			requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
				return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
			})

			zigDB := db.(*ZigCoreDB)
			vectorQuery := benchmarkVectorValues(0, vectorCase.dim)
			var totalNS uint64
			var lookupNS uint64
			var searchNS uint64
			var hitsNS uint64
			var fallbackNS uint64
			var hbcTotalNS uint64
			var hbcSetupNS uint64
			var hbcRootNS uint64
			var hbcNodeMissNS uint64
			var hbcQuantMissNS uint64
			var hbcExpandNS uint64
			var hbcLeafNS uint64
			var hbcRerankNS uint64
			var hbcRerankLoadNS uint64
			var hbcRerankDistNS uint64
			var hbcNodes uint64
			var hbcLeaves uint64
			var hbcReranked uint64

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				profile, err := zigDB.bridge.SearchDenseProfile("dense_idx", vectorQuery, 10, 10, 0)
				if err != nil {
					b.Fatal(err)
				}
				totalNS += profile.TotalNS
				lookupNS += profile.IndexLookupNS
				searchNS += profile.SearchNS
				hitsNS += profile.HitsNS
				fallbackNS += profile.FallbackNS
				hbcTotalNS += profile.HBCTotalNS
				hbcSetupNS += profile.HBCSetupNS
				hbcRootNS += profile.HBCRootLoadNS
				hbcNodeMissNS += profile.HBCNodeMissNS
				hbcQuantMissNS += profile.HBCQuantMissNS
				hbcExpandNS += profile.HBCExpandNS
				hbcLeafNS += profile.HBCLeafNS
				hbcRerankNS += profile.HBCRerankNS
				hbcRerankLoadNS += profile.HBCRerankLoadNS
				hbcRerankDistNS += profile.HBCRerankDistNS
				hbcNodes += profile.HBCNodes
				hbcLeaves += profile.HBCLeaves
				hbcReranked += profile.HBCReranked
			}
			if b.N > 0 {
				denom := float64(b.N)
				b.ReportMetric(float64(totalNS)/denom, "zig_total_ns/op")
				b.ReportMetric(float64(lookupNS)/denom, "zig_lookup_ns/op")
				b.ReportMetric(float64(searchNS)/denom, "zig_search_ns/op")
				b.ReportMetric(float64(hitsNS)/denom, "zig_hits_ns/op")
				b.ReportMetric(float64(fallbackNS)/denom, "zig_fallback_ns/op")
				b.ReportMetric(float64(hbcTotalNS)/denom, "zig_hbc_total_ns/op")
				b.ReportMetric(float64(hbcSetupNS)/denom, "zig_hbc_setup_ns/op")
				b.ReportMetric(float64(hbcRootNS)/denom, "zig_hbc_root_ns/op")
				b.ReportMetric(float64(hbcNodeMissNS)/denom, "zig_hbc_node_miss_ns/op")
				b.ReportMetric(float64(hbcQuantMissNS)/denom, "zig_hbc_quant_miss_ns/op")
				b.ReportMetric(float64(hbcExpandNS)/denom, "zig_hbc_expand_ns/op")
				b.ReportMetric(float64(hbcLeafNS)/denom, "zig_hbc_leaf_ns/op")
				b.ReportMetric(float64(hbcRerankNS)/denom, "zig_hbc_rerank_ns/op")
				b.ReportMetric(float64(hbcRerankLoadNS)/denom, "zig_hbc_rerank_load_ns/op")
				b.ReportMetric(float64(hbcRerankDistNS)/denom, "zig_hbc_rerank_dist_ns/op")
				b.ReportMetric(float64(hbcNodes)/denom, "zig_hbc_nodes/op")
				b.ReportMetric(float64(hbcLeaves)/denom, "zig_hbc_leaves/op")
				b.ReportMetric(float64(hbcReranked)/denom, "zig_hbc_reranked/op")
			}
		})
	}
}

func BenchmarkZigDenseVectorSearchWireProfile(b *testing.B) {
	tableSchema := benchmarkSchema()
	backend := benchmarkBackend{
		name: "zig",
		open: func(b *testing.B, tableSchema *schema.TableSchema) DB {
			b.Helper()
			db := NewZigCoreDB(zap.NewNop(), nil, tableSchema, map[string]indexes.IndexConfig{}, nil, nil, nil)
			requireOpenBenchmarkDB(b, db, tableSchema)
			return db
		},
	}

	for _, vectorCase := range benchmarkVectorCases() {
		vectorCase := vectorCase
		b.Run(vectorCase.name, func(b *testing.B) {
			db := backend.open(b, tableSchema)
			requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
			requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
				Field:          "embedding",
				Dimension:      vectorCase.dim,
				DistanceMetric: indexes.DistanceMetricL2Squared,
			}))
			requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
				return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
			})

			zigDB := db.(*ZigCoreDB)
			reqBytes := encodeSearchWireDenseRequest("dense_idx", benchmarkVectorValues(0, vectorCase.dim), 10, 10, 0)
			var totalNS uint64
			var decodeNS uint64
			var searchNS uint64
			var resolveNS uint64
			var encodeNS uint64
			var fallbackNS uint64
			var hbcTotalNS uint64
			var hbcSetupNS uint64
			var hbcRootNS uint64
			var hbcNodeMissNS uint64
			var hbcQuantMissNS uint64
			var hbcExpandNS uint64
			var hbcLeafNS uint64
			var hbcRerankNS uint64
			var hbcRerankLoadNS uint64
			var hbcRerankDistNS uint64

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, profile, err := zigDB.bridge.SearchDenseWireProfile(reqBytes)
				if err != nil {
					b.Fatal(err)
				}
				totalNS += profile.TotalNS
				decodeNS += profile.DecodeNS
				searchNS += profile.SearchNS
				resolveNS += profile.ResolveNS
				encodeNS += profile.EncodeNS
				fallbackNS += profile.FallbackNS
				hbcTotalNS += profile.HBCTotalNS
				hbcSetupNS += profile.HBCSetupNS
				hbcRootNS += profile.HBCRootLoadNS
				hbcNodeMissNS += profile.HBCNodeMissNS
				hbcQuantMissNS += profile.HBCQuantMissNS
				hbcExpandNS += profile.HBCExpandNS
				hbcLeafNS += profile.HBCLeafNS
				hbcRerankNS += profile.HBCRerankNS
				hbcRerankLoadNS += profile.HBCRerankLoadNS
				hbcRerankDistNS += profile.HBCRerankDistNS
			}
			if b.N > 0 {
				denom := float64(b.N)
				b.ReportMetric(float64(totalNS)/denom, "zig_wire_total_ns/op")
				b.ReportMetric(float64(decodeNS)/denom, "zig_wire_decode_ns/op")
				b.ReportMetric(float64(searchNS)/denom, "zig_wire_search_ns/op")
				b.ReportMetric(float64(resolveNS)/denom, "zig_wire_resolve_ns/op")
				b.ReportMetric(float64(encodeNS)/denom, "zig_wire_encode_ns/op")
				b.ReportMetric(float64(fallbackNS)/denom, "zig_wire_fallback_ns/op")
				b.ReportMetric(float64(hbcTotalNS)/denom, "zig_wire_hbc_total_ns/op")
				b.ReportMetric(float64(hbcSetupNS)/denom, "zig_wire_hbc_setup_ns/op")
				b.ReportMetric(float64(hbcRootNS)/denom, "zig_wire_hbc_root_ns/op")
				b.ReportMetric(float64(hbcNodeMissNS)/denom, "zig_wire_hbc_node_miss_ns/op")
				b.ReportMetric(float64(hbcQuantMissNS)/denom, "zig_wire_hbc_quant_miss_ns/op")
				b.ReportMetric(float64(hbcExpandNS)/denom, "zig_wire_hbc_expand_ns/op")
				b.ReportMetric(float64(hbcLeafNS)/denom, "zig_wire_hbc_leaf_ns/op")
				b.ReportMetric(float64(hbcRerankNS)/denom, "zig_wire_hbc_rerank_ns/op")
				b.ReportMetric(float64(hbcRerankLoadNS)/denom, "zig_wire_hbc_rerank_load_ns/op")
				b.ReportMetric(float64(hbcRerankDistNS)/denom, "zig_wire_hbc_rerank_dist_ns/op")
			}
		})
	}
}

func BenchmarkGoDenseVectorSearchProfile(b *testing.B) {
	tableSchema := benchmarkSchema()

	for _, vectorCase := range benchmarkVectorCases() {
		vectorCase := vectorCase
		b.Run(vectorCase.name, func(b *testing.B) {
			db := NewDBImpl(zap.NewNop(), nil, tableSchema, map[string]indexes.IndexConfig{}, nil, nil, nil)
			requireOpenBenchmarkDB(b, db, tableSchema)
			requireAddIndex(b, db, *indexes.NewFullTextIndexConfig("full_text_index", false))
			requireAddIndex(b, db, *indexes.NewEmbeddingsConfig("dense_idx", indexes.EmbeddingsIndexConfig{
				Field:          "embedding",
				Dimension:      vectorCase.dim,
				DistanceMetric: indexes.DistanceMetricL2Squared,
			}))
			requireSeedDocs(b, db, vectorCase.count, Op_SyncLevelEmbeddings, func(i int) []byte {
				return benchmarkDenseDocJSON("dense_idx", i, vectorCase.dim)
			})

			req := &indexes.RemoteIndexSearchRequest{
				VectorSearches: map[string]vector.T{
					"dense_idx": benchmarkVectorValues(0, vectorCase.dim),
				},
				Limit: 10,
			}
			reqBytes, err := json.Marshal(req)
			if err != nil {
				b.Fatal(err)
			}

			var totalNS uint64
			var rootNS uint64
			var nodeMissNS uint64
			var quantMissNS uint64
			var expandNS uint64
			var leafNS uint64
			var nodes uint64
			var leaves uint64
			var approxNodes uint64
			var approxLeaves uint64
			var approxVectors uint64
			var exactVectors uint64
			var reranked uint64
			var rerankLoadNS uint64
			var rerankDistNS uint64
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Search(ctx, reqBytes); err != nil {
					b.Fatal(err)
				}
				profile := vectorindex.LastHBCDebugSearchProfile()
				totalNS += profile.TotalNS
				rootNS += profile.RootLoadNS
				nodeMissNS += profile.NodeCacheMissNS
				quantMissNS += profile.QuantizedCacheMissNS
				expandNS += profile.ChildExpandNS
				leafNS += profile.LeafScoreNS
				nodes += profile.NodesVisited
				leaves += profile.LeavesExplored
				approxNodes += profile.ApproxNodesExpanded
				approxLeaves += profile.ApproxLeavesScored
				approxVectors += profile.ApproxVectorsScored
				exactVectors += profile.ExactVectorsScored
				reranked += profile.RerankedVectors
				rerankLoadNS += profile.RerankVectorLoadNS
				rerankDistNS += profile.RerankDistanceNS
			}
			if b.N > 0 {
				denom := float64(b.N)
				b.ReportMetric(float64(totalNS)/denom, "go_hbc_total_ns/op")
				b.ReportMetric(float64(rootNS)/denom, "go_hbc_root_ns/op")
				b.ReportMetric(float64(nodeMissNS)/denom, "go_hbc_node_miss_ns/op")
				b.ReportMetric(float64(quantMissNS)/denom, "go_hbc_quant_miss_ns/op")
				b.ReportMetric(float64(expandNS)/denom, "go_hbc_expand_ns/op")
				b.ReportMetric(float64(leafNS)/denom, "go_hbc_leaf_ns/op")
				b.ReportMetric(float64(nodes)/denom, "go_hbc_nodes/op")
				b.ReportMetric(float64(leaves)/denom, "go_hbc_leaves/op")
				b.ReportMetric(float64(approxNodes)/denom, "go_hbc_approx_nodes/op")
				b.ReportMetric(float64(approxLeaves)/denom, "go_hbc_approx_leaves/op")
				b.ReportMetric(float64(approxVectors)/denom, "go_hbc_approx_vectors/op")
				b.ReportMetric(float64(exactVectors)/denom, "go_hbc_exact_vectors/op")
				b.ReportMetric(float64(reranked)/denom, "go_hbc_reranked/op")
				b.ReportMetric(float64(rerankLoadNS)/denom, "go_hbc_rerank_load_ns/op")
				b.ReportMetric(float64(rerankDistNS)/denom, "go_hbc_rerank_dist_ns/op")
				b.ReportMetric(float64(totalNS-rootNS-nodeMissNS-quantMissNS-expandNS-leafNS-rerankLoadNS-rerankDistNS)/denom, "go_hbc_other_ns/op")
			}
		})
	}
}
