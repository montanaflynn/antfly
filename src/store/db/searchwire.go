package db

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/blevesearch/bleve/v2"
)

const (
	searchWireMagic   uint32 = 0x41464442 // AFDB
	searchWireVersion uint16 = 1

	searchWireOpDenseKnn  uint16 = 1
	searchWireOpTextMatch uint16 = 2
)

type searchWireDenseRequest struct {
	IndexName string
	Vector    []float32
	K         uint32
	Limit     uint32
	Offset    uint32
}

type searchWireTextMatchRequest struct {
	IndexName string
	Field     string
	Text      string
	Limit     uint32
	Offset    uint32
}

var errSearchWireInvalid = errors.New("invalid search wire payload")

func searchWireOp(raw []byte) (uint16, bool) {
	if len(raw) < 12 {
		return 0, false
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != searchWireMagic {
		return 0, false
	}
	if binary.LittleEndian.Uint16(raw[4:6]) != searchWireVersion {
		return 0, false
	}
	return binary.LittleEndian.Uint16(raw[6:8]), true
}

func encodeSearchWireDenseRequest(indexName string, vector []float32, k, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 4 + 2 + 2
	out := make([]byte, headerLen+len(indexName)+len(vector)*4)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], searchWireMagic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], searchWireVersion)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], searchWireOpDenseKnn)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], k)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(vector)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	for _, value := range vector {
		binary.LittleEndian.PutUint32(out[cursor:], math.Float32bits(value))
		cursor += 4
	}
	return out
}

func decodeSearchWireDenseRequest(raw []byte) (searchWireDenseRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 4 + 2 + 2
	if len(raw) < headerLen {
		return searchWireDenseRequest{}, errSearchWireInvalid
	}
	if op, ok := searchWireOp(raw); !ok || op != searchWireOpDenseKnn {
		return searchWireDenseRequest{}, errSearchWireInvalid
	}
	k := binary.LittleEndian.Uint32(raw[12:16])
	limit := binary.LittleEndian.Uint32(raw[16:20])
	offset := binary.LittleEndian.Uint32(raw[20:24])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[24:26]))
	dims := int(binary.LittleEndian.Uint16(raw[26:28]))
	if len(raw) < headerLen+indexNameLen+dims*4 {
		return searchWireDenseRequest{}, errSearchWireInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	vector := make([]float32, dims)
	for i := range vector {
		vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[cursor : cursor+4]))
		cursor += 4
	}
	return searchWireDenseRequest{
		IndexName: indexName,
		Vector:    vector,
		K:         k,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func encodeSearchWireTextMatchRequest(indexName, field, text string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(text))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], searchWireMagic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], searchWireVersion)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], searchWireOpTextMatch)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(text)))
	cursor += 4
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], text)
	return out
}

func decodeSearchWireTextMatchRequest(raw []byte) (searchWireTextMatchRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	if len(raw) < headerLen {
		return searchWireTextMatchRequest{}, errSearchWireInvalid
	}
	if op, ok := searchWireOp(raw); !ok || op != searchWireOpTextMatch {
		return searchWireTextMatchRequest{}, errSearchWireInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	textLen := int(binary.LittleEndian.Uint32(raw[24:28]))
	if len(raw) < headerLen+indexNameLen+fieldLen+textLen {
		return searchWireTextMatchRequest{}, errSearchWireInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	text := string(raw[cursor : cursor+textLen])
	return searchWireTextMatchRequest{
		IndexName: indexName,
		Field:     field,
		Text:      text,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func encodeSearchWireVectorResponse(result *vectorindex.SearchResult) []byte {
	if result == nil {
		return encodeSearchWireHitsResponse(searchWireOpDenseKnn, 0, nil)
	}
	hits := make([]searchWireHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}
		hits = append(hits, searchWireHit{ID: hit.ID, Score: hit.Score})
	}
	return encodeSearchWireHitsResponse(searchWireOpDenseKnn, uint32(result.Total), hits)
}

func encodeSearchWireBleveResponse(result *bleve.SearchResult) []byte {
	if result == nil {
		return encodeSearchWireHitsResponse(searchWireOpTextMatch, 0, nil)
	}
	hits := make([]searchWireHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}
		hits = append(hits, searchWireHit{ID: hit.ID, Score: float32(hit.Score)})
	}
	return encodeSearchWireHitsResponse(searchWireOpTextMatch, uint32(result.Total), hits)
}

type searchWireHit struct {
	ID    string
	Score float32
}

func encodeSearchWireHitsResponse(op uint16, totalHits uint32, hits []searchWireHit) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4
	const hitLen = 4 + 2 + 2 + 4
	idsLen := 0
	for _, hit := range hits {
		idsLen += len(hit.ID)
	}
	out := make([]byte, headerLen+len(hits)*hitLen+idsLen)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], searchWireMagic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], searchWireVersion)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], op)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], totalHits)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(hits)))
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], uint32(idsLen))
	cursor += 4

	hitCursor := cursor
	idsCursor := headerLen + len(hits)*hitLen
	blobOffset := 0
	for _, hit := range hits {
		binary.LittleEndian.PutUint32(out[hitCursor:], uint32(blobOffset))
		hitCursor += 4
		binary.LittleEndian.PutUint16(out[hitCursor:], uint16(len(hit.ID)))
		hitCursor += 2
		binary.LittleEndian.PutUint16(out[hitCursor:], 0)
		hitCursor += 2
		binary.LittleEndian.PutUint32(out[hitCursor:], math.Float32bits(hit.Score))
		hitCursor += 4
		copy(out[idsCursor:], hit.ID)
		idsCursor += len(hit.ID)
		blobOffset += len(hit.ID)
	}
	return out
}
