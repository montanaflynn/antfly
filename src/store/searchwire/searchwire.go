package searchwire

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/antflydb/antfly/lib/vectorindex"
	"github.com/blevesearch/bleve/v2"
)

const (
	ContentType = "application/x-antfly-search-wire"

	Magic   uint32 = 0x41464442 // AFDB
	Version uint16 = 1

	OpDenseKnn        uint16 = 1
	OpTextMatch       uint16 = 2
	OpTextTerm        uint16 = 3
	OpTextMatchPhrase uint16 = 4
	OpTextQueryString uint16 = 5
	OpTextBool        uint16 = 6
	OpTextPrefix      uint16 = 7
	OpTextWildcard    uint16 = 8
	OpTextRegexp      uint16 = 9
	OpTextFuzzy       uint16 = 10
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
	Op    uint16
	Field string
	Text  string
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
		if len(raw) < cursor+fieldLen+textLen {
			return nil, 0, ErrInvalid
		}
		field := string(raw[cursor : cursor+fieldLen])
		cursor += fieldLen
		text := string(raw[cursor : cursor+textLen])
		cursor += textLen
		clauses[i] = TextClause{Op: op, Field: field, Text: text}
	}
	return clauses, cursor, nil
}

func validClauseOp(op uint16) bool {
	switch op {
	case OpTextMatch, OpTextTerm, OpTextMatchPhrase, OpTextQueryString:
		return true
	case OpTextPrefix, OpTextWildcard, OpTextRegexp:
		return true
	default:
		return false
	}
}
