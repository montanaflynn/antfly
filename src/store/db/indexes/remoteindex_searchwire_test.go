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
	"time"

	"github.com/antflydb/antfly/lib/types"
	"github.com/antflydb/antfly/lib/vector"
	"github.com/antflydb/antfly/src/store/searchwire"
	"github.com/blevesearch/bleve/v2"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
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

func TestRemoteIndexSearchInContext_UsesWireForQueryString(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewQueryStringQuery(`body:hello`), 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextQueryString, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body: io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextQueryString, 1, []remoteIndexWireHit{
				{id: "doc-1", score: 1.0},
			}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForSimpleBool(t *testing.T) {
	must := query.NewMatchQuery("hello")
	must.SetField("body")
	should := query.NewTermQuery("world")
	should.SetField("body")
	boolQ := query.NewBooleanQuery([]query.Query{must}, []query.Query{should}, nil)
	req := bleve.NewSearchRequestOptions(boolQ, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextBool, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextBool, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForPrefix(t *testing.T) {
	q := query.NewPrefixQuery("he")
	q.SetField("body")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextPrefix, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextPrefix, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForFuzzy(t *testing.T) {
	q := query.NewFuzzyQuery("helo")
	q.SetField("body")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextFuzzy, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextFuzzy, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForBoolWithFuzzyClause(t *testing.T) {
	must := query.NewFuzzyQuery("helo")
	must.SetField("body")
	boolQ := query.NewBooleanQuery([]query.Query{must}, nil, nil)
	req := bleve.NewSearchRequestOptions(boolQ, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextBool, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextBool, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForBoolWithGeoClauses(t *testing.T) {
	mustDistance := bleve.NewGeoDistanceQuery(-122.4194, 37.7749, "2km")
	mustDistance.SetField("location")
	mustShape, err := query.NewGeoShapeQuery([][][][]float64{
		{{{-122.6, 37.9}, {-122.2, 37.9}, {-122.2, 37.7}, {-122.6, 37.7}, {-122.6, 37.9}}},
	}, "polygon", "intersects")
	require.NoError(t, err)
	mustShape.SetField("location")
	boolQ := query.NewBooleanQuery([]query.Query{mustDistance, mustShape}, nil, nil)
	req := bleve.NewSearchRequestOptions(boolQ, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextBool, binary.LittleEndian.Uint16(body[6:8]))
		boolReq, err := searchwire.DecodeTextBoolRequest(body)
		require.NoError(t, err)
		require.Len(t, boolReq.Must, 2)
		require.Equal(t, searchWireOpTextGeoDistance, boolReq.Must[0].Op)
		require.Equal(t, "location", boolReq.Must[0].Field)
		require.Equal(t, "2km", boolReq.Must[0].Distance)
		require.Equal(t, searchWireOpTextGeoShape, boolReq.Must[1].Op)
		require.Equal(t, "intersects", boolReq.Must[1].Relation)
		require.Len(t, boolReq.Must[1].ShapePolygons, 1)

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextBool, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForNestedBool(t *testing.T) {
	must := query.NewMatchQuery("alpha")
	must.SetField("content")
	should := query.NewPrefixQuery("sm")
	should.SetField("title")
	nested := query.NewBooleanQuery([]query.Query{must}, []query.Query{should}, nil)
	boolQ := query.NewBooleanQuery([]query.Query{nested}, nil, nil)
	req := bleve.NewSearchRequestOptions(boolQ, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		boolReq, err := searchwire.DecodeTextBoolRequest(body)
		require.NoError(t, err)
		require.Len(t, boolReq.Must, 1)
		require.Equal(t, searchWireOpTextBool, boolReq.Must[0].Op)
		require.Len(t, boolReq.Must[0].Must, 1)
		require.Equal(t, searchWireOpTextMatch, boolReq.Must[0].Must[0].Op)
		require.Len(t, boolReq.Must[0].Should, 1)
		require.Equal(t, searchWireOpTextPrefix, boolReq.Must[0].Should[0].Op)

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextBool, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForDisjunctionMinShould(t *testing.T) {
	left := query.NewMatchQuery("hello")
	left.SetField("body")
	right := query.NewMatchQuery("world")
	right.SetField("body")
	disj := query.NewDisjunctionQuery([]query.Query{left, right})
	disj.SetMin(2)

	body, op, ok := encodeSimpleTextSearchWire(bleve.NewSearchRequestOptions(disj, 10, 0, false))
	require.True(t, ok)
	require.Equal(t, searchWireOpTextBool, op)

	boolReq, err := searchwire.DecodeTextBoolRequest(body)
	require.NoError(t, err)
	require.Equal(t, uint16(2), boolReq.MinShould)
	require.Len(t, boolReq.Should, 2)
	require.Equal(t, searchWireOpTextMatch, boolReq.Should[0].Op)
	require.Equal(t, searchWireOpTextMatch, boolReq.Should[1].Op)
}

func TestRemoteIndexSearchInContext_UsesWireForMatchAll(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewMatchAllQuery(), 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextMatchAll, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextMatchAll, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForMatchNone(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewMatchNoneQuery(), 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextMatchNone, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextMatchNone, 0, nil))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(0), res.Total)
	require.Len(t, res.Hits, 0)
}

func TestRemoteIndexSearchInContext_UsesWireForDateRangeString(t *testing.T) {
	q := query.NewDateRangeStringQuery("2025-01-01", "2025-01-02")
	q.SetField("published_at")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextDateRange, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextDateRange, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForNumericRange(t *testing.T) {
	min := 10.0
	max := 20.0
	q := query.NewNumericRangeQuery(&min, &max)
	q.SetField("price")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextNumericRange, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextNumericRange, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForGeoDistance(t *testing.T) {
	q := bleve.NewGeoDistanceQuery(-122.4194, 37.7749, "2km")
	q.SetField("location")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextGeoDistance, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextGeoDistance, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForGeoBoundingBox(t *testing.T) {
	q := bleve.NewGeoBoundingBoxQuery(-122.6, 37.9, -122.2, 37.7)
	q.SetField("location")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextGeoBBox, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextGeoBBox, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForGeoBoundingPolygon(t *testing.T) {
	q := query.NewGeoBoundingPolygonQuery([]blevegeo.Point{
		{Lon: 0, Lat: 0},
		{Lon: 10, Lat: 0},
		{Lon: 10, Lat: 10},
		{Lon: 0, Lat: 10},
	})
	q.SetField("location")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextGeoPolygon, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextGeoPolygon, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForGeoShapePolygon(t *testing.T) {
	q, err := query.NewGeoShapeQuery([][][][]float64{
		{
			{
				{0, 0},
				{10, 0},
				{10, 10},
				{0, 10},
				{0, 0},
			},
		},
	}, "polygon", "intersects")
	require.NoError(t, err)
	q.SetField("location")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextGeoShape, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextGeoShape, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForTermRange(t *testing.T) {
	inclusiveMin := true
	inclusiveMax := false
	q := query.NewTermRangeInclusiveQuery("beta", "gamma", &inclusiveMin, &inclusiveMax)
	q.SetField("title")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextTermRange, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextTermRange, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForDocID(t *testing.T) {
	req := bleve.NewSearchRequestOptions(query.NewDocIDQuery([]string{"doc-1", "doc-3"}), 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextDocID, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextDocID, 2, []remoteIndexWireHit{{id: "doc-1", score: 1.0}, {id: "doc-3", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(2), res.Total)
	require.Len(t, res.Hits, 2)
}

func TestRemoteIndexSearchInContext_UsesWireForBoolField(t *testing.T) {
	q := query.NewBoolFieldQuery(true)
	q.SetField("active")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextBoolField, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextBoolField, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForPhrase(t *testing.T) {
	q := query.NewPhraseQuery([]string{"alpha", "beta"}, "content")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextPhrase, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextPhrase, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForPhraseWithAutoFuzziness(t *testing.T) {
	q := query.NewPhraseQuery([]string{"alpha", "beta"}, "content")
	q.SetAutoFuzziness(true)
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextPhrase, binary.LittleEndian.Uint16(body[6:8]))
		phraseReq, err := searchwire.DecodeTextPhraseRequest(body)
		require.NoError(t, err)
		require.True(t, phraseReq.Auto)

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextPhrase, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
}

func TestRemoteIndexSearchInContext_UsesWireForDateRange(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	q := query.NewDateRangeQuery(start, end)
	q.SetField("published_at")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextDateRange, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextDateRange, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForMultiPhrase(t *testing.T) {
	q := query.NewMultiPhraseQuery([][]string{{"alpha", "beta"}, {"gamma"}}, "content")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextMultiPhrase, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextMultiPhrase, 1, []remoteIndexWireHit{{id: "doc-1", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(1), res.Total)
	require.Len(t, res.Hits, 1)
	require.Equal(t, "doc-1", res.Hits[0].ID)
}

func TestRemoteIndexSearchInContext_UsesWireForIPRange(t *testing.T) {
	q := query.NewIPRangeQuery("192.168.1.0/24")
	q.SetField("ip")
	req := bleve.NewSearchRequestOptions(q, 10, 0, false)

	client := &http.Client{Transport: remoteIndexRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Helper()
		require.Equal(t, searchWireContentType, r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, searchWireMagic, binary.LittleEndian.Uint32(body[0:4]))
		require.Equal(t, searchWireOpTextIPRange, binary.LittleEndian.Uint16(body[6:8]))

		header := make(http.Header)
		header.Set("Content-Type", searchWireContentType)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(makeRemoteIndexWireResponse(searchWireOpTextIPRange, 2, []remoteIndexWireHit{{id: "doc-1", score: 1.0}, {id: "doc-2", score: 1.0}}))),
		}, nil
	})}

	idx, err := NewRemoteIndex(client, []string{"http://wire-search"}, types.ID(1))
	require.NoError(t, err)

	res, err := idx.SearchInContext(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint64(2), res.Total)
	require.Len(t, res.Hits, 2)
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
