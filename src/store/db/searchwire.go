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
	searchWireOpTextQueryString uint16 = searchwire.OpTextQueryString
	searchWireOpTextBool        uint16 = searchwire.OpTextBool
	searchWireOpTextPrefix      uint16 = searchwire.OpTextPrefix
	searchWireOpTextWildcard    uint16 = searchwire.OpTextWildcard
	searchWireOpTextRegexp      uint16 = searchwire.OpTextRegexp
	searchWireOpTextFuzzy       uint16 = searchwire.OpTextFuzzy
	searchWireOpTextMatchAll    uint16 = searchwire.OpTextMatchAll
	searchWireOpTextMatchNone   uint16 = searchwire.OpTextMatchNone
)

type searchWireDenseRequest = searchwire.DenseRequest
type searchWireTextMatchRequest = searchwire.TextRequest
type searchWireTextTermRequest = searchwire.TextRequest
type searchWireTextMatchPhraseRequest = searchwire.TextRequest
type searchWireTextQueryStringRequest = searchwire.TextRequest
type searchWireTextPrefixRequest = searchwire.TextRequest
type searchWireTextWildcardRequest = searchwire.TextRequest
type searchWireTextRegexpRequest = searchwire.TextRequest
type searchWireTextFuzzyRequest = searchwire.TextFuzzyRequest
type searchWireTextClause = searchwire.TextClause
type searchWireTextBoolRequest = searchwire.TextBoolRequest
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

func encodeSearchWireTextQueryStringRequest(indexName, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextQueryStringRequest(indexName, text, limit, offset)
}

func encodeSearchWireTextPrefixRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextPrefixRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextWildcardRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextWildcardRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextRegexpRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextRegexpRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextFuzzyRequest(indexName, field, text string, prefix, fuzziness uint16, auto bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextFuzzyRequest(indexName, field, text, prefix, fuzziness, auto, limit, offset)
}

func encodeSearchWireTextMatchAllRequest(indexName string, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchAllRequest(indexName, limit, offset)
}

func encodeSearchWireTextMatchNoneRequest(indexName string, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchNoneRequest(indexName, limit, offset)
}

func encodeSearchWireTextBoolRequest(indexName string, must, should, mustNot []searchWireTextClause, limit, offset uint32) []byte {
	return searchwire.EncodeTextBoolRequest(indexName, must, should, mustNot, limit, offset)
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

func decodeSearchWireTextQueryStringRequest(raw []byte) (searchWireTextQueryStringRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextQueryString)
}

func decodeSearchWireTextPrefixRequest(raw []byte) (searchWireTextPrefixRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextPrefix)
}

func decodeSearchWireTextWildcardRequest(raw []byte) (searchWireTextWildcardRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextWildcard)
}

func decodeSearchWireTextRegexpRequest(raw []byte) (searchWireTextRegexpRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextRegexp)
}

func decodeSearchWireTextFuzzyRequest(raw []byte) (searchWireTextFuzzyRequest, error) {
	return searchwire.DecodeTextFuzzyRequest(raw)
}

func decodeSearchWireTextMatchAllRequest(raw []byte) (searchWireTextMatchRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextMatchAll)
}

func decodeSearchWireTextMatchNoneRequest(raw []byte) (searchWireTextMatchRequest, error) {
	return searchwire.DecodeTextRequest(raw, searchwire.OpTextMatchNone)
}

func decodeSearchWireTextBoolRequest(raw []byte) (searchWireTextBoolRequest, error) {
	return searchwire.DecodeTextBoolRequest(raw)
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
