package repolib

import (
	"sort"
	"strings"
	"unicode"

	"vibe-tree/backend/internal/repolib/searchdb"
)

type rerankedHit struct {
	hit   searchdb.Hit
	score float64
}

func rerankSearchHits(query string, hits []searchdb.Hit) []searchdb.Hit {
	tokens := searchQueryTokens(query)
	if len(tokens) == 0 || len(hits) <= 1 {
		return hits
	}

	ranked := make([]rerankedHit, 0, len(hits))
	for _, hit := range hits {
		score := hit.Score
		titleText := normalizeComparable(hit.Title)
		excerptText := normalizeComparable(hit.TextExcerpt)
		titleOverlap := tokenOverlapScore(tokens, titleText)
		excerptOverlap := tokenOverlapScore(tokens, excerptText)
		score += titleOverlap * 0.18
		score += excerptOverlap * 0.08
		if hit.SourceKind == "card" && titleOverlap > 0 {
			score += 0.03
		}
		if hit.SourceKind == "card" && excerptOverlap >= 0.45 {
			score += 0.02
		}
		ranked = append(ranked, rerankedHit{
			hit:   hit,
			score: score,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			leftRank := sourceKindRank(ranked[i].hit.SourceKind)
			rightRank := sourceKindRank(ranked[j].hit.SourceKind)
			if leftRank == rightRank {
				return ranked[i].hit.Title < ranked[j].hit.Title
			}
			return leftRank < rightRank
		}
		return ranked[i].score > ranked[j].score
	})

	out := make([]searchdb.Hit, 0, len(ranked))
	for _, item := range ranked {
		item.hit.Score = item.score
		out = append(out, item.hit)
	}
	return out
}

func sourceKindRank(kind string) int {
	switch strings.TrimSpace(kind) {
	case "card":
		return 0
	case "report_section":
		return 1
	case "evidence":
		return 2
	default:
		return 3
	}
}

func tokenOverlapScore(tokens []string, haystack string) float64 {
	if len(tokens) == 0 || strings.TrimSpace(haystack) == "" {
		return 0
	}
	matched := 0
	for _, token := range tokens {
		if token != "" && strings.Contains(haystack, token) {
			matched++
		}
	}
	return float64(matched) / float64(len(tokens))
}

func normalizeComparable(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func searchQueryTokens(raw string) []string {
	raw = normalizeComparable(raw)
	if raw == "" {
		return nil
	}

	type tokenKind int

	const (
		tokenOther tokenKind = iota
		tokenASCII
		tokenHan
	)

	flush := func(kind tokenKind, runes []rune, out *[]string) {
		if len(runes) == 0 {
			return
		}
		token := string(runes)
		switch kind {
		case tokenASCII:
			*out = append(*out, token)
		case tokenHan:
			if len(runes) >= 2 {
				*out = append(*out, token)
				if len(runes) > 2 {
					for idx := 0; idx < len(runes)-1; idx++ {
						*out = append(*out, string(runes[idx:idx+2]))
					}
				}
			}
		}
	}

	currentKind := tokenOther
	current := make([]rune, 0, len(raw))
	out := []string{}
	for _, r := range raw {
		nextKind := tokenOther
		switch {
		case unicode.Is(unicode.Han, r):
			nextKind = tokenHan
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			nextKind = tokenASCII
		}
		if nextKind == tokenOther {
			flush(currentKind, current, &out)
			currentKind = tokenOther
			current = current[:0]
			continue
		}
		if currentKind != tokenOther && currentKind != nextKind {
			flush(currentKind, current, &out)
			current = current[:0]
		}
		currentKind = nextKind
		current = append(current, r)
	}
	flush(currentKind, current, &out)
	return uniqueTrimmed(out)
}
