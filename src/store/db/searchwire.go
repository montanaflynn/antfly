package db

import (
	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/antflydb/antfly/src/store/searchwire"
	"github.com/blevesearch/bleve/v2"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
)

const (
	searchWireMagic   uint32 = searchwire.Magic
	searchWireVersion uint16 = searchwire.Version

	searchWireOpDenseKnn         uint16 = searchwire.OpDenseKnn
	searchWireOpTextMatch        uint16 = searchwire.OpTextMatch
	searchWireOpTextTerm         uint16 = searchwire.OpTextTerm
	searchWireOpTextMatchPhrase  uint16 = searchwire.OpTextMatchPhrase
	searchWireOpTextQueryString  uint16 = searchwire.OpTextQueryString
	searchWireOpTextBool         uint16 = searchwire.OpTextBool
	searchWireOpTextPrefix       uint16 = searchwire.OpTextPrefix
	searchWireOpTextWildcard     uint16 = searchwire.OpTextWildcard
	searchWireOpTextRegexp       uint16 = searchwire.OpTextRegexp
	searchWireOpTextFuzzy        uint16 = searchwire.OpTextFuzzy
	searchWireOpTextMatchAll     uint16 = searchwire.OpTextMatchAll
	searchWireOpTextMatchNone    uint16 = searchwire.OpTextMatchNone
	searchWireOpTextDateRange    uint16 = searchwire.OpTextDateRange
	searchWireOpTextNumericRange uint16 = searchwire.OpTextNumericRange
	searchWireOpTextGeoDistance  uint16 = searchwire.OpTextGeoDistance
	searchWireOpTextGeoBBox      uint16 = searchwire.OpTextGeoBBox
	searchWireOpTextGeoPolygon   uint16 = searchwire.OpTextGeoPolygon
	searchWireOpTextTermRange    uint16 = searchwire.OpTextTermRange
	searchWireOpTextDocID        uint16 = searchwire.OpTextDocID
	searchWireOpTextBoolField    uint16 = searchwire.OpTextBoolField
	searchWireOpTextIPRange      uint16 = searchwire.OpTextIPRange
	searchWireOpTextPhrase       uint16 = searchwire.OpTextPhrase
	searchWireOpTextMultiPhrase  uint16 = searchwire.OpTextMultiPhrase
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
type searchWireTextDateRangeRequest = searchwire.TextDateRangeRequest
type searchWireTextNumericRangeRequest = searchwire.TextNumericRangeRequest
type searchWireTextGeoDistanceRequest = searchwire.TextGeoDistanceRequest
type searchWireTextGeoBoundingBoxRequest = searchwire.TextGeoBoundingBoxRequest
type searchWireTextGeoBoundingPolygonRequest = searchwire.TextGeoBoundingPolygonRequest
type searchWireTextTermRangeRequest = searchwire.TextTermRangeRequest
type searchWireTextDocIDRequest = searchwire.TextDocIDRequest
type searchWireTextBoolFieldRequest = searchwire.TextBoolFieldRequest
type searchWireTextIPRangeRequest = searchwire.TextIPRangeRequest
type searchWireTextPhraseRequest = searchwire.TextPhraseRequest
type searchWireTextMultiPhraseRequest = searchwire.TextMultiPhraseRequest
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

func encodeSearchWireTextMatchRequest(indexName, field, text, analyzer string, prefix, fuzziness uint16, auto bool, operator uint8, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchRequest(indexName, field, text, analyzer, prefix, fuzziness, auto, operator, limit, offset)
}

func encodeSearchWireTextTermRequest(indexName, field, text string, limit, offset uint32) []byte {
	return searchwire.EncodeTextTermRequest(indexName, field, text, limit, offset)
}

func encodeSearchWireTextMatchPhraseRequest(indexName, field, text, analyzer string, fuzziness uint16, auto bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextMatchPhraseRequest(indexName, field, text, analyzer, fuzziness, auto, limit, offset)
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

func encodeSearchWireTextDateRangeRequest(indexName, field, start, end string, inclusiveStart, inclusiveEnd *bool, parser string, limit, offset uint32) []byte {
	return searchwire.EncodeTextDateRangeRequest(indexName, field, start, end, inclusiveStart, inclusiveEnd, parser, limit, offset)
}

func encodeSearchWireTextNumericRangeRequest(indexName, field string, min, max *float64, inclusiveMin, inclusiveMax *bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextNumericRangeRequest(indexName, field, min, max, inclusiveMin, inclusiveMax, limit, offset)
}

func encodeSearchWireTextGeoDistanceRequest(indexName, field string, lon, lat float64, distance string, limit, offset uint32) []byte {
	return searchwire.EncodeTextGeoDistanceRequest(indexName, field, lon, lat, distance, limit, offset)
}

func encodeSearchWireTextGeoBoundingBoxRequest(indexName, field string, topLeftLon, topLeftLat, bottomRightLon, bottomRightLat float64, limit, offset uint32) []byte {
	return searchwire.EncodeTextGeoBoundingBoxRequest(indexName, field, topLeftLon, topLeftLat, bottomRightLon, bottomRightLat, limit, offset)
}

func encodeSearchWireTextGeoBoundingPolygonRequest(indexName, field string, points []blevegeo.Point, limit, offset uint32) []byte {
	return searchwire.EncodeTextGeoBoundingPolygonRequest(indexName, field, points, limit, offset)
}

func encodeSearchWireTextTermRangeRequest(indexName, field, min, max string, inclusiveMin, inclusiveMax *bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextTermRangeRequest(indexName, field, min, max, inclusiveMin, inclusiveMax, limit, offset)
}

func encodeSearchWireTextDocIDRequest(ids []string, limit, offset uint32) []byte {
	return searchwire.EncodeTextDocIDRequest(ids, limit, offset)
}

func encodeSearchWireTextBoolFieldRequest(indexName, field string, value bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextBoolFieldRequest(indexName, field, value, limit, offset)
}

func encodeSearchWireTextIPRangeRequest(indexName, field, cidr string, limit, offset uint32) []byte {
	return searchwire.EncodeTextIPRangeRequest(indexName, field, cidr, limit, offset)
}

func encodeSearchWireTextPhraseRequest(indexName, field string, terms []string, fuzziness uint16, auto bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextPhraseRequest(indexName, field, terms, fuzziness, auto, limit, offset)
}

func encodeSearchWireTextMultiPhraseRequest(indexName, field string, terms [][]string, fuzziness uint16, auto bool, limit, offset uint32) []byte {
	return searchwire.EncodeTextMultiPhraseRequest(indexName, field, terms, fuzziness, auto, limit, offset)
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

func decodeSearchWireTextDateRangeRequest(raw []byte) (searchWireTextDateRangeRequest, error) {
	return searchwire.DecodeTextDateRangeRequest(raw)
}

func decodeSearchWireTextNumericRangeRequest(raw []byte) (searchWireTextNumericRangeRequest, error) {
	return searchwire.DecodeTextNumericRangeRequest(raw)
}

func decodeSearchWireTextGeoDistanceRequest(raw []byte) (searchWireTextGeoDistanceRequest, error) {
	return searchwire.DecodeTextGeoDistanceRequest(raw)
}

func decodeSearchWireTextGeoBoundingBoxRequest(raw []byte) (searchWireTextGeoBoundingBoxRequest, error) {
	return searchwire.DecodeTextGeoBoundingBoxRequest(raw)
}

func decodeSearchWireTextGeoBoundingPolygonRequest(raw []byte) (searchWireTextGeoBoundingPolygonRequest, error) {
	return searchwire.DecodeTextGeoBoundingPolygonRequest(raw)
}

func decodeSearchWireTextTermRangeRequest(raw []byte) (searchWireTextTermRangeRequest, error) {
	return searchwire.DecodeTextTermRangeRequest(raw)
}

func decodeSearchWireTextDocIDRequest(raw []byte) (searchWireTextDocIDRequest, error) {
	return searchwire.DecodeTextDocIDRequest(raw)
}

func decodeSearchWireTextBoolFieldRequest(raw []byte) (searchWireTextBoolFieldRequest, error) {
	return searchwire.DecodeTextBoolFieldRequest(raw)
}

func decodeSearchWireTextIPRangeRequest(raw []byte) (searchWireTextIPRangeRequest, error) {
	return searchwire.DecodeTextIPRangeRequest(raw)
}

func decodeSearchWireTextPhraseRequest(raw []byte) (searchWireTextPhraseRequest, error) {
	return searchwire.DecodeTextPhraseRequest(raw)
}

func decodeSearchWireTextMultiPhraseRequest(raw []byte) (searchWireTextMultiPhraseRequest, error) {
	return searchwire.DecodeTextMultiPhraseRequest(raw)
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
