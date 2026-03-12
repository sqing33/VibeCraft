package searchdb

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

func packFloat32LE(values []float32) ([]byte, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out, nil
}

func buildFilterWhere(analysisFilters []string, sourceKinds []string) (string, []any) {
	args := []any{}
	clauses := []string{}
	if len(analysisFilters) > 0 {
		placeholders := make([]string, 0, len(analysisFilters))
		for _, id := range uniqueTrimmed(analysisFilters) {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		if len(placeholders) > 0 {
			clauses = append(clauses, "AND c.analysis_id IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if len(sourceKinds) > 0 {
		placeholders := make([]string, 0, len(sourceKinds))
		for _, kind := range uniqueTrimmed(sourceKinds) {
			placeholders = append(placeholders, "?")
			args = append(args, kind)
		}
		if len(placeholders) > 0 {
			clauses = append(clauses, "AND c.source_kind IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " " + strings.Join(clauses, " "), args
}

func buildVecFilterWhere(analysisFilters []string, sourceKinds []string) (string, []any) {
	args := []any{}
	clauses := []string{}
	if len(analysisFilters) > 0 {
		placeholders := make([]string, 0, len(analysisFilters))
		for _, id := range uniqueTrimmed(analysisFilters) {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		if len(placeholders) > 0 {
			clauses = append(clauses, "AND analysis_id IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if len(sourceKinds) > 0 {
		placeholders := make([]string, 0, len(sourceKinds))
		for _, kind := range uniqueTrimmed(sourceKinds) {
			placeholders = append(placeholders, "?")
			args = append(args, kind)
		}
		if len(placeholders) > 0 {
			clauses = append(clauses, "AND source_kind IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " " + strings.Join(clauses, " "), args
}

func splitFlatRefs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	out := []string{}
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func maxInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

type agg struct {
	hit   Hit
	score float64
}

func sortAgg(items []agg) {
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
}

func uniqueTrimmed(values []string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
