package db

import (
	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/antflydb/antfly/src/store/searchwire"
	"github.com/blevesearch/bleve/v2"
)

const (
	searchWireMagic   uint32 = searchwire.Magic
	searchWireVersion uint16 = searchwire.Version

	searchWireOpDenseKnn        uint16 = searchwire.OpDenseKnn
	searchWireOpTextMatch       uint16 = searchwire.OpTextMatch
	searchWireOpTextTerm        uint16 = searchwire.OpTextTerm
	searchWireOpTextMatchPhrase uint16 = searchwire.OpTextMatchPhrase
)

type searchWireDenseRequest = searchwire.DenseRequest
type searchWireTextMatchRequest = searchwire.TextRequest
type searchWireTextTermRequest = searchwire.TextRequest
type searchWireTextMatchPhraseRequest = searchwire.TextRequest
type searchWireHit = searchwire.Hit

var errSearchWireInvalid = searchwire.ErrInvalid

func searchWireOp(raw []byte) (uint16, bool) {
	return searchwire.Op(raw)
}

func encodeSearchWireDenseRequest(indexName string, vector []float32, k, limit, offset uint32) []byte {
	return searchwire.EncodeDenseRequest(indexName, vector, k, limit, offset)
}

func decodeSearchWireDenseRequest(raw []byte) (searchWireDenseRequest, error) {
	return searchwire.DecodeDenseRequest(raw)
}

func encodeSearchWireTextMatchRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextTermRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextTermRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextMatchPhraseRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchPhraseRequest(indexName, field, text, limit, offset)
}

func decodeSearchWireTextMatchRequest(raw []byte) (searchWireTextMatchRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextMatch)
}

func decodeSearchWireTextTermRequest(raw []byte) (searchWireTextTermRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextTerm)
}

func decodeSearchWireTextMatchPhraseRequest(raw []byte) (searchWireTextMatchPhraseRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextMatchPhrase)
}

func encodeSearchWireVectorResponse(result *vectorindex.SearchResult) []byte {
	return searchwire.EncodeVectorResponse(result)
}

func encodeSearchWireBleveResponse(result *bleve.SearchResult) []byte {
	return searchwire.EncodeBleveResponseForOp(searchwire.OpTextMatch, result)
}

func encodeSearchWireBleveResponseForOp(op uint16, result *bleve.SearchResult) []byte {
	return searchwire.EncodeBleveResponseForOp(op, result)
}

func encodeSearchWireHitsResponse(op uint16, totalHits uint32, hits []searchWireHit) []byte {
	return searchwire.EncodeHitsResponse(op, totalHits, hits)
}
