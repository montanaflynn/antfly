package indexes

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/lib/vector"
	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
)

type remoteIndexWireHit struct {
	id    string
	score float32
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func encodeRemoteIndexWireResponse(op uint16, total uint32, hits []remoteIndexWireHit) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4
	const hitLen = 4 + 2 + 2 + 4

	idsLen := 0
	for _, hit := range hits {
		idsLen += len(hit.id)
	}

	out := make([]byte, headerLen+len(hits)*hitLen+idsLen)
	binary.LittleEndian.PutUint32(out[0:4], searchWireMagic)
	binary.LittleEndian.PutUint16(out[4:6], searchWireVersion)
	binary.LittleEndian.PutUint16(out[6:8], op)
	binary.LittleEndian.PutUint32(out[8:12], total)
	binary.LittleEndian.PutUint32(out[12:16], uint32(len(hits)))
	binary.LittleEndian.PutUint32(out[16:20], uint32(idsLen))

	idsOffset := 0
	hitsBase := headerLen
	idsBase := headerLen + len(hits)*hitLen
	for i, hit := range hits {
		base := hitsBase + i*hitLen
		binary.LittleEndian.PutUint32(out[base:base+4], uint32(idsOffset))
		binary.LittleEndian.PutUint16(out[base+4:base+6], uint16(len(hit.id)))
		binary.LittleEndian.PutUint32(out[base+8:base+12], math.Float32bits(hit.score))
		copy(out[idsBase+idsOffset:], hit.id)
		idsOffset += len(hit.id)
	}
	return out
}

func remoteIndexJSONBleveSearch(
	ctx context.Context,
	r *RemoteIndex,
	req *bleve.SearchRequest,
) (*bleve.SearchResult, error) {
	version := uint32(0)
	if r.schema != nil {
		version = r.schema.Version
	}
	riReq := RemoteIndexSearchRequest{BleveSearchRequest: req, FullTextIndexVersion: version}
	reqBytes, err := json.Marshal(riReq)
	if err != nil {
		return nil, err
	}
	hreq, err := commonShardRequest(r, reqBytes, "application/json", ctx)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var result RemoteIndexSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.BleveSearchResult, nil
}

func remoteIndexJSONDenseSearch(
	ctx context.Context,
	r *RemoteIndex,
	req *RemoteIndexSearchRequest,
) (*RemoteIndexSearchResult, error) {
	version := uint32(0)
	if r.schema != nil {
		version = r.schema.Version
	}
	req.FullTextIndexVersion = version
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hreq, err := commonShardRequest(r, reqBytes, "application/json", ctx)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var result RemoteIndexSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func commonShardRequest(r *RemoteIndex, body []byte, contentType string, ctx context.Context) (*http.Request, error) {
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.urls[0]+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", contentType)
	return hreq, nil
}

func BenchmarkRemoteIndexSearchProtocols(b *testing.B) {
	textReq := bleve.NewSearchRequestOptions(query.NewMatchQuery("hello"), 10, 0, false)
	textReq.Query.(*query.MatchQuery).SetField("body")
	textJSONResp, err := json.Marshal(&RemoteIndexSearchResult{
		Total: 2,
		BleveSearchResult: &bleve.SearchResult{
			Status:   &bleve.SearchStatus{Total: 1, Successful: 1},
			Request:  textReq,
			Total:    2,
			MaxScore: 1.0,
			Hits: search.DocumentMatchCollection{
				&search.DocumentMatch{ID: "doc-1", Score: 1.0},
				&search.DocumentMatch{ID: "doc-2", Score: 0.7},
			},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	textWireResp := encodeRemoteIndexWireResponse(searchWireOpTextMatch, 2, []remoteIndexWireHit{
		{id: "doc-1", score: 1.0},
		{id: "doc-2", score: 0.7},
	})
	nestedMust := query.NewMatchQuery("hello")
	nestedMust.SetField("body")
	nestedShould := query.NewPrefixQuery("wo")
	nestedShould.SetField("body")
	nestedBoolReq := bleve.NewSearchRequestOptions(query.NewBooleanQuery([]query.Query{
		query.NewBooleanQuery([]query.Query{nestedMust}, []query.Query{nestedShould}, nil),
	}, nil, nil), 10, 0, false)
	nestedBoolJSONResp, err := json.Marshal(&RemoteIndexSearchResult{
		Total: 2,
		BleveSearchResult: &bleve.SearchResult{
			Status:   &bleve.SearchStatus{Total: 1, Successful: 1},
			Request:  nestedBoolReq,
			Total:    2,
			MaxScore: 1.0,
			Hits: search.DocumentMatchCollection{
				&search.DocumentMatch{ID: "doc-1", Score: 1.0},
				&search.DocumentMatch{ID: "doc-2", Score: 0.7},
			},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	nestedBoolWireResp := encodeRemoteIndexWireResponse(searchWireOpTextBool, 2, []remoteIndexWireHit{
		{id: "doc-1", score: 1.0},
		{id: "doc-2", score: 0.7},
	})

	denseReq := &RemoteIndexSearchRequest{
		Limit: 10,
		VectorSearches: map[string]vector.T{
			"embeddings_index": {0.1, 0.2},
		},
	}
	denseJSONResp, err := json.Marshal(&RemoteIndexSearchResult{
		Total: 2,
		VectorSearchResult: map[string]*vectorindex.SearchResult{
			"embeddings_index": {
				Total: 2,
				Status: &vectorindex.SearchStatus{
					Total:      2,
					Successful: 2,
				},
				Hits: []*vectorindex.SearchHit{
					{ID: "doc-1", Distance: 0.11, Score: 0.11},
					{ID: "doc-2", Distance: 0.23, Score: 0.23},
				},
			},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	denseWireResp := encodeRemoteIndexWireResponse(searchWireOpDenseKnn, 2, []remoteIndexWireHit{
		{id: "doc-1", score: 0.11},
		{id: "doc-2", score: 0.23},
	})

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer func() { _ = r.Body.Close() }()
		body, _ := io.ReadAll(r.Body)
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, searchWireContentType) {
			op := binary.LittleEndian.Uint16(body[6:8])
			respBody := textWireResp
			switch op {
			case searchWireOpDenseKnn:
				respBody = denseWireResp
			case searchWireOpTextMatch:
				respBody = textWireResp
			case searchWireOpTextBool:
				respBody = nestedBoolWireResp
			default:
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("unsupported wire op")),
				}, nil
			}
			header := make(http.Header)
			header.Set("Content-Type", searchWireContentType)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}

		var req RemoteIndexSearchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(err.Error())),
			}, nil
		}
		respBody := textJSONResp
		if req.BleveSearchRequest != nil {
			if _, ok := req.BleveSearchRequest.Query.(*query.BooleanQuery); ok {
				respBody = nestedBoolJSONResp
			}
		}
		if len(req.VectorSearches) > 0 {
			respBody = denseJSONResp
		}
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://search-wire-bench"}, types.ID(1))
	if err != nil {
		b.Fatal(err)
	}

	b.Run("TextMatch/JSON", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := remoteIndexJSONBleveSearch(context.Background(), idx, textReq); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TextMatch/Wire", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := idx.SearchInContext(context.Background(), textReq); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Dense/JSON", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reqCopy := *denseReq
			reqCopy.VectorSearches = map[string]vector.T{
				"embeddings_index": {0.1, 0.2},
			}
			if _, err := remoteIndexJSONDenseSearch(context.Background(), idx, &reqCopy); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Dense/Wire", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reqCopy := *denseReq
			reqCopy.VectorSearches = map[string]vector.T{
				"embeddings_index": {0.1, 0.2},
			}
			if _, err := idx.RemoteSearch(context.Background(), &reqCopy); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NestedBool/JSON", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := remoteIndexJSONBleveSearch(context.Background(), idx, nestedBoolReq); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NestedBool/Wire", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := idx.SearchInContext(context.Background(), nestedBoolReq); err != nil {
				b.Fatal(err)
			}
		}
	})
}
