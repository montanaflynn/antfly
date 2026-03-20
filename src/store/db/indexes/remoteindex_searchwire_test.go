package indexes

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/lib/vector"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/stretchr/testify/require"
)

type remoteIndexRoundTripFunc func(*http.Request) (*http.Response, error)

func (f remoteIndexRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func makeRemoteIndexWireResponse(op uint16, total uint32, hits []remoteIndexWireHit) []byte {
	return encodeRemoteIndexWireResponse(op, total, hits)
}

func TestRemoteIndexSearchInContext_UsesWireForSimpleMatch(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewMatchQuery("hello"), 10, 0, false)
	req.Query.(*query.MatchQuery).SetField("body")

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextMatch, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body: io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextMatch, 2, []remoteIndexWireHit{
				{id: "doc-1", score: 1.0},
				{id: "doc-2", score: 0.7},
			}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(2), res.Total)
	require.Len(t, res.Hits, 2)
	require.Equal(t, "doc-1", res.Hits[0].ID)
	require.InDelta(t, 1.0, res.Hits[0].Score, 1e-6)
}

func TestRemoteIndexRemoteSearch_UsesWireForDense(t *testing.T) {
	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpDenseKnn, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body: io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpDenseKnn, 2, []remoteIndexWireHit{
				{id: "doc-1", score: 0.11},
				{id: "doc-2", score: 0.23},
			}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.RemoteSearch(context.Background(), &RemoteIndexSearchRequest{
		Limit: 10,
		VectorSearches: map[string]vector.T{
			"embeddings_index": {0.1, 0.2},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(2), res.Total)
	require.Contains(t, res.VectorSearchResult, "embeddings_index")
	require.Len(t, res.VectorSearchResult["embeddings_index"].Hits, 2)
	require.Equal(t, "doc-1", res.VectorSearchResult["embeddings_index"].Hits[0].ID)
	require.InDelta(t, 0.11, res.VectorSearchResult["embeddings_index"].Hits[0].Score, 1e-6)
}

func TestRemoteIndexSearchInContext_FallsBackToJSONForUnsupportedQuery(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewMatchQuery("hello"), 10, 0, false)
	req.Query.(*query.MatchQuery).SetField("body")
	respBody, err := json.Marshal(&RemoteIndexSearchResult{
		Total: 1,
		BleveSearchResult: &bleve.SearchResult{
			Status:   &bleve.SearchStatus{Total: 1, Successful: 1},
			Request:  req,
			Total:    1,
			MaxScore: 1.0,
			Hits: search.DocumentMatchCollection{
				&search.DocumentMatch{ID: "doc-1", Score: 1.0},
			},
		},
	})
	require.NoError(t, err)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"))
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://json-search"}, types.ID(1))
	require.NoError(t, err)
	idx.q = &FieldFilter{Fields: []string{"title"}}

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
}
