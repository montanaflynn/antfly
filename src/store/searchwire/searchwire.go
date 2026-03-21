package searchwire

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/blevesearch/bleve/v2"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
)

const (
	ContentType = "application/x-antfly-search-wire"

	Magic   uint32 = 0x41464442 // AFDB
	Version uint16 = 1

	OpDenseKnn         uint16 = 1
	OpTextMatch        uint16 = 2
	OpTextTerm         uint16 = 3
	OpTextMatchPhrase  uint16 = 4
	OpTextQueryString  uint16 = 5
	OpTextBool         uint16 = 6
	OpTextPrefix       uint16 = 7
	OpTextWildcard     uint16 = 8
	OpTextRegexp       uint16 = 9
	OpTextFuzzy        uint16 = 10
	OpTextMatchAll     uint16 = 11
	OpTextMatchNone    uint16 = 12
	OpTextDateRange    uint16 = 13
	OpTextNumericRange uint16 = 14
	OpTextGeoDistance  uint16 = 15
	OpTextGeoBBox      uint16 = 16
	OpTextGeoPolygon   uint16 = 17
	OpTextTermRange    uint16 = 18
	OpTextDocID        uint16 = 19
	OpTextBoolField    uint16 = 20
	OpTextIPRange      uint16 = 21
)

var ErrInvalid = errors.New("invalid search wire payload")

type DenseRequest struct {
	IndexName string
	Vector    []float32
	K         uint32
	Limit     uint32
	Offset    uint32
}

type TextRequest struct {
	IndexName string
	Field     string
	Text      string
	Limit     uint32
	Offset    uint32
}

type TextClause struct {
	Op        uint16
	Field     string
	Text      string
	Prefix    uint16
	Fuzziness uint16
	Auto      bool
}

type TextBoolRequest struct {
	IndexName string
	Must      []TextClause
	Should    []TextClause
	MustNot   []TextClause
	Limit     uint32
	Offset    uint32
}

type TextFuzzyRequest struct {
	IndexName string
	Field     string
	Text      string
	Prefix    uint16
	Fuzziness uint16
	Auto      bool
	Limit     uint32
	Offset    uint32
}

type TextDateRangeRequest struct {
	IndexName      string
	Field          string
	Start          string
	End            string
	InclusiveStart *bool
	InclusiveEnd   *bool
	DateTimeParser string
	Limit          uint32
	Offset         uint32
}

type TextNumericRangeRequest struct {
	IndexName    string
	Field        string
	Min          *float64
	Max          *float64
	InclusiveMin *bool
	InclusiveMax *bool
	Limit        uint32
	Offset       uint32
}

type TextGeoDistanceRequest struct {
	IndexName string
	Field     string
	Lon       float64
	Lat       float64
	Distance  string
	Limit     uint32
	Offset    uint32
}

type TextGeoBoundingBoxRequest struct {
	IndexName      string
	Field          string
	TopLeftLon     float64
	TopLeftLat     float64
	BottomRightLon float64
	BottomRightLat float64
	Limit          uint32
	Offset         uint32
}

type TextGeoBoundingPolygonRequest struct {
	IndexName string
	Field     string
	Points    []blevegeo.Point
	Limit     uint32
	Offset    uint32
}

type TextTermRangeRequest struct {
	IndexName    string
	Field        string
	Min          string
	Max          string
	InclusiveMin *bool
	InclusiveMax *bool
	Limit        uint32
	Offset       uint32
}

type TextDocIDRequest struct {
	IDs    []string
	Limit  uint32
	Offset uint32
}

type TextBoolFieldRequest struct {
	IndexName string
	Field     string
	Value     bool
	Limit     uint32
	Offset    uint32
}

type TextIPRangeRequest struct {
	IndexName string
	Field     string
	CIDR      string
	Limit     uint32
	Offset    uint32
}

type Hit struct {
	ID    string
	Score float32
}

func Op(raw []byte) (uint16, bool) {
	if len(raw) < 12 {
		return 0, false
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != Magic {
		return 0, false
	}
	if binary.LittleEndian.Uint16(raw[4:6]) != Version {
		return 0, false
	}
	return binary.LittleEndian.Uint16(raw[6:8]), true
}

func EncodeDenseRequest(indexName string, vector []float32, k, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 4 + 2 + 2
	out := make([]byte, headerLen+len(indexName)+len(vector)*4)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpDenseKnn)
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

func DecodeDenseRequest(raw []byte) (DenseRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 4 + 2 + 2
	if len(raw) < headerLen {
		return DenseRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpDenseKnn {
		return DenseRequest{}, ErrInvalid
	}
	k := binary.LittleEndian.Uint32(raw[12:16])
	limit := binary.LittleEndian.Uint32(raw[16:20])
	offset := binary.LittleEndian.Uint32(raw[20:24])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[24:26]))
	dims := int(binary.LittleEndian.Uint16(raw[26:28]))
	if len(raw) < headerLen+indexNameLen+dims*4 {
		return DenseRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	vector := make([]float32, dims)
	for i := range vector {
		vector[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[cursor : cursor+4]))
		cursor += 4
	}
	return DenseRequest{
		IndexName: indexName,
		Vector:    vector,
		K:         k,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func EncodeTextMatchRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextMatch, indexName, field, text, limit, offset)
}

func EncodeTextTermRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextTerm, indexName, field, text, limit, offset)
}

func EncodeTextMatchPhraseRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextMatchPhrase, indexName, field, text, limit, offset)
}

func EncodeTextQueryStringRequest(indexName, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextQueryString, indexName, "", text, limit, offset)
}

func EncodeTextPrefixRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextPrefix, indexName, field, text, limit, offset)
}

func EncodeTextWildcardRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextWildcard, indexName, field, text, limit, offset)
}

func EncodeTextRegexpRequest(indexName, field, text string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextRegexp, indexName, field, text, limit, offset)
}

func EncodeTextFuzzyRequest(indexName, field, text string, prefix, fuzziness uint16, auto bool, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2 + 2 + 1 + 1 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(text))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextFuzzy)
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
	binary.LittleEndian.PutUint16(out[cursor:], prefix)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], fuzziness)
	cursor += 2
	if auto {
		out[cursor] = 1
	}
	cursor += 1
	cursor += 1 // reserved
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(text)))
	cursor += 4
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], text)
	return out
}

func EncodeTextMatchAllRequest(indexName string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextMatchAll, indexName, "", "", limit, offset)
}

func EncodeTextMatchNoneRequest(indexName string, limit, offset uint32) []byte {
	return EncodeTextRequest(OpTextMatchNone, indexName, "", "", limit, offset)
}

func EncodeTextDateRangeRequest(indexName, field, start, end string, inclusiveStart, inclusiveEnd *bool, parser string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4 + 4 + 2
	out := make([]byte, headerLen+len(indexName)+len(field)+len(start)+len(end)+len(parser))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextDateRange)
	cursor += 2
	var flags uint32
	if inclusiveStart != nil {
		flags |= 1 << 0
		if *inclusiveStart {
			flags |= 1 << 1
		}
	}
	if inclusiveEnd != nil {
		flags |= 1 << 2
		if *inclusiveEnd {
			flags |= 1 << 3
		}
	}
	binary.LittleEndian.PutUint32(out[cursor:], flags)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(start)))
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(end)))
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(parser)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], start)
	cursor += len(start)
	copy(out[cursor:], end)
	cursor += len(end)
	copy(out[cursor:], parser)
	return out
}

func EncodeTextNumericRangeRequest(indexName, field string, min, max *float64, inclusiveMin, inclusiveMax *bool, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 2 + 2
	out := make([]byte, headerLen+len(indexName)+len(field))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextNumericRange)
	cursor += 2
	var flags uint32
	if min != nil {
		flags |= 1 << 0
		binary.LittleEndian.PutUint64(out[20:28], math.Float64bits(*min))
	}
	if max != nil {
		flags |= 1 << 1
		binary.LittleEndian.PutUint64(out[28:36], math.Float64bits(*max))
	}
	if inclusiveMin != nil {
		flags |= 1 << 2
		if *inclusiveMin {
			flags |= 1 << 3
		}
	}
	if inclusiveMax != nil {
		flags |= 1 << 4
		if *inclusiveMax {
			flags |= 1 << 5
		}
	}
	binary.LittleEndian.PutUint32(out[cursor:], flags)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	cursor += 8 // min
	cursor += 8 // max
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	return out
}

func EncodeTextGeoDistanceRequest(indexName, field string, lon, lat float64, distance string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 2 + 2 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(distance))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextGeoDistance)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(lon))
	cursor += 8
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(lat))
	cursor += 8
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(distance)))
	cursor += 4
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], distance)
	return out
}

func EncodeTextGeoBoundingBoxRequest(indexName, field string, topLeftLon, topLeftLat, bottomRightLon, bottomRightLat float64, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 8 + 8 + 2 + 2
	out := make([]byte, headerLen+len(indexName)+len(field))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextGeoBBox)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(topLeftLon))
	cursor += 8
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(topLeftLat))
	cursor += 8
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(bottomRightLon))
	cursor += 8
	binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(bottomRightLat))
	cursor += 8
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	return out
}

func EncodeTextGeoBoundingPolygonRequest(indexName, field string, points []blevegeo.Point, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2
	out := make([]byte, headerLen+len(indexName)+len(field)+len(points)*16)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextGeoPolygon)
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
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(points)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	for _, point := range points {
		binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(point.Lon))
		cursor += 8
		binary.LittleEndian.PutUint64(out[cursor:], math.Float64bits(point.Lat))
		cursor += 8
	}
	return out
}

func EncodeTextTermRangeRequest(indexName, field, min, max string, inclusiveMin, inclusiveMax *bool, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(min)+len(max))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextTermRange)
	cursor += 2
	var flags uint32
	if inclusiveMin != nil {
		flags |= 1 << 0
		if *inclusiveMin {
			flags |= 1 << 1
		}
	}
	if inclusiveMax != nil {
		flags |= 1 << 2
		if *inclusiveMax {
			flags |= 1 << 3
		}
	}
	binary.LittleEndian.PutUint32(out[cursor:], flags)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(field)))
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(min)))
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(max)))
	cursor += 4
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], min)
	cursor += len(min)
	copy(out[cursor:], max)
	return out
}

func EncodeTextDocIDRequest(ids []string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2
	totalIDsLen := 0
	for _, id := range ids {
		totalIDsLen += 2 + len(id)
	}
	out := make([]byte, headerLen+totalIDsLen)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextDocID)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(ids)))
	cursor += 2
	for _, id := range ids {
		binary.LittleEndian.PutUint16(out[cursor:], uint16(len(id)))
		cursor += 2
		copy(out[cursor:], id)
		cursor += len(id)
	}
	return out
}

func EncodeTextBoolFieldRequest(indexName, field string, value bool, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 1 + 1
	out := make([]byte, headerLen+len(indexName)+len(field))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextBoolField)
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
	if value {
		out[cursor] = 1
	}
	cursor += 1
	cursor += 1
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	return out
}

func EncodeTextIPRangeRequest(indexName, field, cidr string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(cidr))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextIPRange)
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
	binary.LittleEndian.PutUint32(out[cursor:], uint32(len(cidr)))
	cursor += 4
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	copy(out[cursor:], field)
	cursor += len(field)
	copy(out[cursor:], cidr)
	return out
}

func EncodeTextBoolRequest(indexName string, must, should, mustNot []TextClause, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2 + 2
	totalClausesLen := encodedClausesLen(must) + encodedClausesLen(should) + encodedClausesLen(mustNot)
	out := make([]byte, headerLen+len(indexName)+totalClausesLen)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], OpTextBool)
	cursor += 2
	binary.LittleEndian.PutUint32(out[cursor:], 0)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], limit)
	cursor += 4
	binary.LittleEndian.PutUint32(out[cursor:], offset)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(indexName)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(must)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(should)))
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], uint16(len(mustNot)))
	cursor += 2
	copy(out[cursor:], indexName)
	cursor += len(indexName)
	encodeClauses(out, &cursor, must)
	encodeClauses(out, &cursor, should)
	encodeClauses(out, &cursor, mustNot)
	return out
}

func EncodeTextRequest(op uint16, indexName, field, text string, limit, offset uint32) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	out := make([]byte, headerLen+len(indexName)+len(field)+len(text))
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
	cursor += 2
	binary.LittleEndian.PutUint16(out[cursor:], op)
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

func DecodeTextRequest(raw []byte, opExpected uint16) (TextRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	if len(raw) < headerLen {
		return TextRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != opExpected {
		return TextRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	textLen := int(binary.LittleEndian.Uint32(raw[24:28]))
	if len(raw) < headerLen+indexNameLen+fieldLen+textLen {
		return TextRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	text := string(raw[cursor : cursor+textLen])
	return TextRequest{
		IndexName: indexName,
		Field:     field,
		Text:      text,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextBoolRequest(raw []byte) (TextBoolRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2 + 2
	if len(raw) < headerLen {
		return TextBoolRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextBool {
		return TextBoolRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	mustCount := int(binary.LittleEndian.Uint16(raw[22:24]))
	shouldCount := int(binary.LittleEndian.Uint16(raw[24:26]))
	mustNotCount := int(binary.LittleEndian.Uint16(raw[26:28]))
	if len(raw) < headerLen+indexNameLen {
		return TextBoolRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	must, next, err := decodeClauses(raw, cursor, mustCount)
	if err != nil {
		return TextBoolRequest{}, err
	}
	cursor = next
	should, next, err := decodeClauses(raw, cursor, shouldCount)
	if err != nil {
		return TextBoolRequest{}, err
	}
	cursor = next
	mustNot, next, err := decodeClauses(raw, cursor, mustNotCount)
	if err != nil {
		return TextBoolRequest{}, err
	}
	if next != len(raw) {
		return TextBoolRequest{}, ErrInvalid
	}
	return TextBoolRequest{
		IndexName: indexName,
		Must:      must,
		Should:    should,
		MustNot:   mustNot,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextFuzzyRequest(raw []byte) (TextFuzzyRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2 + 2 + 1 + 1 + 4
	if len(raw) < headerLen {
		return TextFuzzyRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextFuzzy {
		return TextFuzzyRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	prefix := binary.LittleEndian.Uint16(raw[24:26])
	fuzziness := binary.LittleEndian.Uint16(raw[26:28])
	auto := raw[28] != 0
	textLen := int(binary.LittleEndian.Uint32(raw[30:34]))
	if len(raw) < headerLen+indexNameLen+fieldLen+textLen {
		return TextFuzzyRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	text := string(raw[cursor : cursor+textLen])
	return TextFuzzyRequest{
		IndexName: indexName,
		Field:     field,
		Text:      text,
		Prefix:    prefix,
		Fuzziness: fuzziness,
		Auto:      auto,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextDateRangeRequest(raw []byte) (TextDateRangeRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4 + 4 + 2
	if len(raw) < headerLen {
		return TextDateRangeRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextDateRange {
		return TextDateRangeRequest{}, ErrInvalid
	}
	flags := binary.LittleEndian.Uint32(raw[8:12])
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	startLen := int(binary.LittleEndian.Uint32(raw[24:28]))
	endLen := int(binary.LittleEndian.Uint32(raw[28:32]))
	parserLen := int(binary.LittleEndian.Uint16(raw[32:34]))
	if len(raw) < headerLen+indexNameLen+fieldLen+startLen+endLen+parserLen {
		return TextDateRangeRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	start := string(raw[cursor : cursor+startLen])
	cursor += startLen
	end := string(raw[cursor : cursor+endLen])
	cursor += endLen
	parser := string(raw[cursor : cursor+parserLen])
	var inclusiveStart *bool
	var inclusiveEnd *bool
	if flags&(1<<0) != 0 {
		value := flags&(1<<1) != 0
		inclusiveStart = &value
	}
	if flags&(1<<2) != 0 {
		value := flags&(1<<3) != 0
		inclusiveEnd = &value
	}
	return TextDateRangeRequest{
		IndexName:      indexName,
		Field:          field,
		Start:          start,
		End:            end,
		InclusiveStart: inclusiveStart,
		InclusiveEnd:   inclusiveEnd,
		DateTimeParser: parser,
		Limit:          limit,
		Offset:         offset,
	}, nil
}

func DecodeTextNumericRangeRequest(raw []byte) (TextNumericRangeRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 2 + 2
	if len(raw) < headerLen {
		return TextNumericRangeRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextNumericRange {
		return TextNumericRangeRequest{}, ErrInvalid
	}
	flags := binary.LittleEndian.Uint32(raw[8:12])
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	var min *float64
	var max *float64
	if flags&(1<<0) != 0 {
		value := math.Float64frombits(binary.LittleEndian.Uint64(raw[20:28]))
		min = &value
	}
	if flags&(1<<1) != 0 {
		value := math.Float64frombits(binary.LittleEndian.Uint64(raw[28:36]))
		max = &value
	}
	indexNameLen := int(binary.LittleEndian.Uint16(raw[36:38]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[38:40]))
	if len(raw) < headerLen+indexNameLen+fieldLen {
		return TextNumericRangeRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	var inclusiveMin *bool
	var inclusiveMax *bool
	if flags&(1<<2) != 0 {
		value := flags&(1<<3) != 0
		inclusiveMin = &value
	}
	if flags&(1<<4) != 0 {
		value := flags&(1<<5) != 0
		inclusiveMax = &value
	}
	return TextNumericRangeRequest{
		IndexName:    indexName,
		Field:        field,
		Min:          min,
		Max:          max,
		InclusiveMin: inclusiveMin,
		InclusiveMax: inclusiveMax,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

func DecodeTextGeoDistanceRequest(raw []byte) (TextGeoDistanceRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 2 + 2 + 4
	if len(raw) < headerLen {
		return TextGeoDistanceRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextGeoDistance {
		return TextGeoDistanceRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	lon := math.Float64frombits(binary.LittleEndian.Uint64(raw[20:28]))
	lat := math.Float64frombits(binary.LittleEndian.Uint64(raw[28:36]))
	indexNameLen := int(binary.LittleEndian.Uint16(raw[36:38]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[38:40]))
	distanceLen := int(binary.LittleEndian.Uint32(raw[40:44]))
	if len(raw) < headerLen+indexNameLen+fieldLen+distanceLen {
		return TextGeoDistanceRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	distance := string(raw[cursor : cursor+distanceLen])
	return TextGeoDistanceRequest{
		IndexName: indexName,
		Field:     field,
		Lon:       lon,
		Lat:       lat,
		Distance:  distance,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextGeoBoundingBoxRequest(raw []byte) (TextGeoBoundingBoxRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 8 + 8 + 8 + 8 + 2 + 2
	if len(raw) < headerLen {
		return TextGeoBoundingBoxRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextGeoBBox {
		return TextGeoBoundingBoxRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	topLeftLon := math.Float64frombits(binary.LittleEndian.Uint64(raw[20:28]))
	topLeftLat := math.Float64frombits(binary.LittleEndian.Uint64(raw[28:36]))
	bottomRightLon := math.Float64frombits(binary.LittleEndian.Uint64(raw[36:44]))
	bottomRightLat := math.Float64frombits(binary.LittleEndian.Uint64(raw[44:52]))
	indexNameLen := int(binary.LittleEndian.Uint16(raw[52:54]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[54:56]))
	if len(raw) < headerLen+indexNameLen+fieldLen {
		return TextGeoBoundingBoxRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	return TextGeoBoundingBoxRequest{
		IndexName:      indexName,
		Field:          field,
		TopLeftLon:     topLeftLon,
		TopLeftLat:     topLeftLat,
		BottomRightLon: bottomRightLon,
		BottomRightLat: bottomRightLat,
		Limit:          limit,
		Offset:         offset,
	}, nil
}

func DecodeTextGeoBoundingPolygonRequest(raw []byte) (TextGeoBoundingPolygonRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 2
	if len(raw) < headerLen {
		return TextGeoBoundingPolygonRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextGeoPolygon {
		return TextGeoBoundingPolygonRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	pointCount := int(binary.LittleEndian.Uint16(raw[24:26]))
	if len(raw) < headerLen+indexNameLen+fieldLen+pointCount*16 {
		return TextGeoBoundingPolygonRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	points := make([]blevegeo.Point, pointCount)
	for i := range points {
		points[i] = blevegeo.Point{
			Lon: math.Float64frombits(binary.LittleEndian.Uint64(raw[cursor : cursor+8])),
			Lat: math.Float64frombits(binary.LittleEndian.Uint64(raw[cursor+8 : cursor+16])),
		}
		cursor += 16
	}
	return TextGeoBoundingPolygonRequest{
		IndexName: indexName,
		Field:     field,
		Points:    points,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextTermRangeRequest(raw []byte) (TextTermRangeRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4 + 4
	if len(raw) < headerLen {
		return TextTermRangeRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextTermRange {
		return TextTermRangeRequest{}, ErrInvalid
	}
	flags := binary.LittleEndian.Uint32(raw[8:12])
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	minLen := int(binary.LittleEndian.Uint32(raw[24:28]))
	maxLen := int(binary.LittleEndian.Uint32(raw[28:32]))
	if len(raw) < headerLen+indexNameLen+fieldLen+minLen+maxLen {
		return TextTermRangeRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	min := string(raw[cursor : cursor+minLen])
	cursor += minLen
	max := string(raw[cursor : cursor+maxLen])
	var inclusiveMin *bool
	var inclusiveMax *bool
	if flags&(1<<0) != 0 {
		value := flags&(1<<1) != 0
		inclusiveMin = &value
	}
	if flags&(1<<2) != 0 {
		value := flags&(1<<3) != 0
		inclusiveMax = &value
	}
	return TextTermRangeRequest{
		IndexName:    indexName,
		Field:        field,
		Min:          min,
		Max:          max,
		InclusiveMin: inclusiveMin,
		InclusiveMax: inclusiveMax,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

func DecodeTextDocIDRequest(raw []byte) (TextDocIDRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2
	if len(raw) < headerLen {
		return TextDocIDRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextDocID {
		return TextDocIDRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	count := int(binary.LittleEndian.Uint16(raw[20:22]))
	cursor := headerLen
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		if len(raw) < cursor+2 {
			return TextDocIDRequest{}, ErrInvalid
		}
		idLen := int(binary.LittleEndian.Uint16(raw[cursor : cursor+2]))
		cursor += 2
		if len(raw) < cursor+idLen {
			return TextDocIDRequest{}, ErrInvalid
		}
		ids[i] = string(raw[cursor : cursor+idLen])
		cursor += idLen
	}
	return TextDocIDRequest{IDs: ids, Limit: limit, Offset: offset}, nil
}

func DecodeTextBoolFieldRequest(raw []byte) (TextBoolFieldRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 1 + 1
	if len(raw) < headerLen {
		return TextBoolFieldRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextBoolField {
		return TextBoolFieldRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	value := raw[24] != 0
	if len(raw) < headerLen+indexNameLen+fieldLen {
		return TextBoolFieldRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	return TextBoolFieldRequest{
		IndexName: indexName,
		Field:     field,
		Value:     value,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func DecodeTextIPRangeRequest(raw []byte) (TextIPRangeRequest, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4 + 2 + 2 + 4
	if len(raw) < headerLen {
		return TextIPRangeRequest{}, ErrInvalid
	}
	if op, ok := Op(raw); !ok || op != OpTextIPRange {
		return TextIPRangeRequest{}, ErrInvalid
	}
	limit := binary.LittleEndian.Uint32(raw[12:16])
	offset := binary.LittleEndian.Uint32(raw[16:20])
	indexNameLen := int(binary.LittleEndian.Uint16(raw[20:22]))
	fieldLen := int(binary.LittleEndian.Uint16(raw[22:24]))
	cidrLen := int(binary.LittleEndian.Uint32(raw[24:28]))
	if len(raw) < headerLen+indexNameLen+fieldLen+cidrLen {
		return TextIPRangeRequest{}, ErrInvalid
	}
	cursor := headerLen
	indexName := string(raw[cursor : cursor+indexNameLen])
	cursor += indexNameLen
	field := string(raw[cursor : cursor+fieldLen])
	cursor += fieldLen
	cidr := string(raw[cursor : cursor+cidrLen])
	return TextIPRangeRequest{
		IndexName: indexName,
		Field:     field,
		CIDR:      cidr,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func EncodeVectorResponse(result *vectorindex.SearchResult) []byte {
	if result == nil {
		return EncodeHitsResponse(OpDenseKnn, 0, nil)
	}
	hits := make([]Hit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}
		hits = append(hits, Hit{ID: hit.ID, Score: hit.Score})
	}
	return EncodeHitsResponse(OpDenseKnn, uint32(result.Total), hits)
}

func EncodeBleveResponseForOp(op uint16, result *bleve.SearchResult) []byte {
	if result == nil {
		return EncodeHitsResponse(op, 0, nil)
	}
	hits := make([]Hit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}
		hits = append(hits, Hit{ID: hit.ID, Score: float32(hit.Score)})
	}
	return EncodeHitsResponse(op, uint32(result.Total), hits)
}

func EncodeHitsResponse(op uint16, totalHits uint32, hits []Hit) []byte {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4
	const hitLen = 4 + 2 + 2 + 4
	idsLen := 0
	for _, hit := range hits {
		idsLen += len(hit.ID)
	}
	out := make([]byte, headerLen+len(hits)*hitLen+idsLen)
	cursor := 0
	binary.LittleEndian.PutUint32(out[cursor:], Magic)
	cursor += 4
	binary.LittleEndian.PutUint16(out[cursor:], Version)
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

func DecodeHits(raw []byte, expectedOp uint16) (uint64, []Hit, error) {
	const headerLen = 4 + 2 + 2 + 4 + 4 + 4
	const hitLen = 4 + 2 + 2 + 4
	if len(raw) < headerLen {
		return 0, nil, ErrInvalid
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != Magic ||
		binary.LittleEndian.Uint16(raw[4:6]) != Version ||
		binary.LittleEndian.Uint16(raw[6:8]) != expectedOp {
		return 0, nil, ErrInvalid
	}
	totalHits := binary.LittleEndian.Uint32(raw[8:12])
	hitCount := int(binary.LittleEndian.Uint32(raw[12:16]))
	idsLen := int(binary.LittleEndian.Uint32(raw[16:20]))
	if len(raw) < headerLen+hitCount*hitLen+idsLen {
		return 0, nil, ErrInvalid
	}
	hitsStart := headerLen
	idsStart := headerLen + hitCount*hitLen
	idsBlob := raw[idsStart : idsStart+idsLen]
	hits := make([]Hit, hitCount)
	for i := 0; i < hitCount; i++ {
		base := hitsStart + i*hitLen
		idOffset := int(binary.LittleEndian.Uint32(raw[base : base+4]))
		idLen := int(binary.LittleEndian.Uint16(raw[base+4 : base+6]))
		if idOffset < 0 || idOffset+idLen > len(idsBlob) {
			return 0, nil, ErrInvalid
		}
		hits[i] = Hit{
			ID:    string(idsBlob[idOffset : idOffset+idLen]),
			Score: math.Float32frombits(binary.LittleEndian.Uint32(raw[base+8 : base+12])),
		}
	}
	return uint64(totalHits), hits, nil
}

func encodedClausesLen(clauses []TextClause) int {
	total := 0
	for _, clause := range clauses {
		total += 2 + 2 + 4 + len(clause.Field) + len(clause.Text)
		if clause.Op == OpTextFuzzy {
			total += 2 + 2 + 1
		}
	}
	return total
}

func encodeClauses(out []byte, cursor *int, clauses []TextClause) {
	for _, clause := range clauses {
		binary.LittleEndian.PutUint16(out[*cursor:], clause.Op)
		*cursor += 2
		binary.LittleEndian.PutUint16(out[*cursor:], uint16(len(clause.Field)))
		*cursor += 2
		binary.LittleEndian.PutUint32(out[*cursor:], uint32(len(clause.Text)))
		*cursor += 4
		if clause.Op == OpTextFuzzy {
			binary.LittleEndian.PutUint16(out[*cursor:], clause.Prefix)
			*cursor += 2
			binary.LittleEndian.PutUint16(out[*cursor:], clause.Fuzziness)
			*cursor += 2
			if clause.Auto {
				out[*cursor] = 1
			}
			*cursor += 1
		}
		copy(out[*cursor:], clause.Field)
		*cursor += len(clause.Field)
		copy(out[*cursor:], clause.Text)
		*cursor += len(clause.Text)
	}
}

func decodeClauses(raw []byte, cursor int, count int) ([]TextClause, int, error) {
	clauses := make([]TextClause, count)
	for i := 0; i < count; i++ {
		if len(raw) < cursor+8 {
			return nil, 0, ErrInvalid
		}
		op := binary.LittleEndian.Uint16(raw[cursor : cursor+2])
		if !validClauseOp(op) {
			return nil, 0, ErrInvalid
		}
		cursor += 2
		fieldLen := int(binary.LittleEndian.Uint16(raw[cursor : cursor+2]))
		cursor += 2
		textLen := int(binary.LittleEndian.Uint32(raw[cursor : cursor+4]))
		cursor += 4
		var prefix uint16
		var fuzziness uint16
		var auto bool
		if op == OpTextFuzzy {
			if len(raw) < cursor+5 {
				return nil, 0, ErrInvalid
			}
			prefix = binary.LittleEndian.Uint16(raw[cursor : cursor+2])
			cursor += 2
			fuzziness = binary.LittleEndian.Uint16(raw[cursor : cursor+2])
			cursor += 2
			auto = raw[cursor] != 0
			cursor += 1
		}
		if len(raw) < cursor+fieldLen+textLen {
			return nil, 0, ErrInvalid
		}
		field := string(raw[cursor : cursor+fieldLen])
		cursor += fieldLen
		text := string(raw[cursor : cursor+textLen])
		cursor += textLen
		clauses[i] = TextClause{Op: op, Field: field, Text: text, Prefix: prefix, Fuzziness: fuzziness, Auto: auto}
	}
	return clauses, cursor, nil
}

func validClauseOp(op uint16) bool {
	switch op {
	case OpTextMatch, OpTextTerm, OpTextMatchPhrase, OpTextQueryString:
		return true
	case OpTextPrefix, OpTextWildcard, OpTextRegexp:
		return true
	case OpTextFuzzy:
		return true
	default:
		return false
	}
}
