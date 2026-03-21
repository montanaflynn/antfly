package db

import (
	"strings"

	"github.com/antflydb/antfly/lib/schema"
	blevegeo "github.com/blevesearch/bleve/v2/geo"
	blevequery "github.com/blevesearch/bleve/v2/search/query"
	json "github.com/goccy/go-json"
)

func normalizeGeoShapeQueryForGeoPoint(q blevequery.Query, tableSchema *schema.TableSchema) (blevequery.Query, error) {
	if q == nil || tableSchema == nil {
		return q, nil
	}

	switch typed := q.(type) {
	case *blevequery.ConjunctionQuery:
		for i, clause := range typed.Conjuncts {
			normalized, err := normalizeGeoShapeQueryForGeoPoint(clause, tableSchema)
			if err != nil {
				return nil, err
			}
			typed.Conjuncts[i] = normalized
		}
		return typed, nil
	case *blevequery.DisjunctionQuery:
		for i, clause := range typed.Disjuncts {
			normalized, err := normalizeGeoShapeQueryForGeoPoint(clause, tableSchema)
			if err != nil {
				return nil, err
			}
			typed.Disjuncts[i] = normalized
		}
		return typed, nil
	case *blevequery.BooleanQuery:
		var err error
		typed.Must, err = normalizeGeoShapeQueryForGeoPoint(typed.Must, tableSchema)
		if err != nil {
			return nil, err
		}
		typed.Should, err = normalizeGeoShapeQueryForGeoPoint(typed.Should, tableSchema)
		if err != nil {
			return nil, err
		}
		typed.MustNot, err = normalizeGeoShapeQueryForGeoPoint(typed.MustNot, tableSchema)
		if err != nil {
			return nil, err
		}
		typed.Filter, err = normalizeGeoShapeQueryForGeoPoint(typed.Filter, tableSchema)
		if err != nil {
			return nil, err
		}
		return typed, nil
	case *blevequery.GeoShapeQuery:
		return rewriteGeoShapeQueryForGeoPoint(typed, tableSchema)
	default:
		return q, nil
	}
}

func rewriteGeoShapeQueryForGeoPoint(q *blevequery.GeoShapeQuery, tableSchema *schema.TableSchema) (blevequery.Query, error) {
	if q == nil || q.Field() == "" || q.Geometry.Shape == nil {
		return q, nil
	}
	if !schemaHasGeoPointField(tableSchema, q.Field()) {
		return q, nil
	}

	relation := q.Geometry.Relation
	if relation == "" {
		relation = "intersects"
	}
	if relation != "intersects" && relation != "within" {
		return q, nil
	}

	raw, err := q.Geometry.Shape.Value()
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	switch strings.ToLower(parsed.Type) {
	case "polygon":
		var coordinates [][][]float64
		if err := json.Unmarshal(parsed.Coordinates, &coordinates); err != nil || len(coordinates) == 0 {
			return q, nil
		}
		polygon, ok := normalizeGeoShapePolygonPoints(coordinates[0])
		if !ok {
			return q, nil
		}
		pq := blevequery.NewGeoBoundingPolygonQuery(polygon)
		pq.SetField(q.Field())
		pq.SetBoost(q.Boost())
		return pq, nil
	case "multipolygon":
		var coordinates [][][][]float64
		if err := json.Unmarshal(parsed.Coordinates, &coordinates); err != nil || len(coordinates) == 0 {
			return q, nil
		}
		disjuncts := make([]blevequery.Query, 0, len(coordinates))
		for _, polygonCoords := range coordinates {
			if len(polygonCoords) == 0 {
				return q, nil
			}
			polygon, ok := normalizeGeoShapePolygonPoints(polygonCoords[0])
			if !ok {
				return q, nil
			}
			pq := blevequery.NewGeoBoundingPolygonQuery(polygon)
			pq.SetField(q.Field())
			disjuncts = append(disjuncts, pq)
		}
		dq := blevequery.NewDisjunctionQuery(disjuncts)
		dq.SetBoost(q.Boost())
		return dq, nil
	default:
		return q, nil
	}
}

func normalizeGeoShapePolygonPoints(coords [][]float64) ([]blevegeo.Point, bool) {
	if len(coords) < 3 {
		return nil, false
	}
	points := make([]blevegeo.Point, 0, len(coords))
	for _, coord := range coords {
		if len(coord) != 2 {
			return nil, false
		}
		points = append(points, blevegeo.Point{Lon: coord[0], Lat: coord[1]})
	}
	return points, true
}

func schemaHasGeoPointField(tableSchema *schema.TableSchema, field string) bool {
	if tableSchema == nil || field == "" {
		return false
	}
	fieldPath := strings.Split(field, ".")
	for docType := range tableSchema.DocumentSchemas {
		if schema.HasAntflyType(tableSchema, docType, fieldPath, schema.AntflyTypeGeopoint) {
			return true
		}
	}
	return false
}
