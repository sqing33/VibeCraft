package repolib

import (
	"context"
	"math"
	"sort"
	"strings"

	"vibecraft/backend/internal/repolib/searchdb"
	"vibecraft/backend/internal/store"
)

type cardSearchResult struct {
	Card         store.RepoKnowledgeCard
	Evidence     []store.RepoKnowledgeEvidence
	RawScore     float64
	DisplayScore float64
	MatchSources []string
	RepositoryID string
	AnalysisID   string
}

type cardSearchAggregate struct {
	cardID       string
	rawScore     float64
	maxScore     float64
	hitCount     int
	topByKind    map[string]float64
	matchSources map[string]struct{}
	repositoryID string
	analysisID   string
}

func (s *Service) collapseSearchHitsToCards(ctx context.Context, hits []searchdb.Hit, topK int) []cardSearchResult {
	if s == nil || s.store == nil || len(hits) == 0 {
		return nil
	}
	aggByCard := map[string]*cardSearchAggregate{}
	for _, hit := range hits {
		cardID := strings.TrimSpace(hit.SourceRefID)
		if cardID == "" {
			continue
		}
		agg, ok := aggByCard[cardID]
		if !ok {
			agg = &cardSearchAggregate{
				cardID:       cardID,
				rawScore:     hit.Score,
				maxScore:     hit.Score,
				topByKind:    map[string]float64{},
				matchSources: map[string]struct{}{},
				repositoryID: hit.RepoSourceID,
				analysisID:   hit.AnalysisID,
			}
			aggByCard[cardID] = agg
		}
		agg.hitCount++
		if hit.Score > agg.maxScore {
			agg.maxScore = hit.Score
			agg.repositoryID = hit.RepoSourceID
			agg.analysisID = hit.AnalysisID
		}
		if kind := strings.TrimSpace(hit.SourceKind); kind != "" {
			agg.matchSources[kind] = struct{}{}
			if hit.Score > agg.topByKind[kind] {
				agg.topByKind[kind] = hit.Score
			}
		}
	}
	aggregates := make([]*cardSearchAggregate, 0, len(aggByCard))
	for _, agg := range aggByCard {
		agg.rawScore = aggregateCardScore(agg)
		aggregates = append(aggregates, agg)
	}
	sort.SliceStable(aggregates, func(i, j int) bool {
		if aggregates[i].rawScore == aggregates[j].rawScore {
			return aggregates[i].cardID < aggregates[j].cardID
		}
		return aggregates[i].rawScore > aggregates[j].rawScore
	})

	results := make([]cardSearchResult, 0, minInt(topK, len(aggregates)))
	for _, agg := range aggregates {
		card, err := s.store.GetRepoCard(ctx, agg.cardID)
		if err != nil {
			continue
		}
		evidence, err := s.store.ListRepoEvidenceByCard(ctx, agg.cardID)
		if err != nil {
			evidence = nil
		}
		if len(evidence) > 3 {
			evidence = evidence[:3]
		}
		results = append(results, cardSearchResult{
			Card:         card,
			Evidence:     evidence,
			RawScore:     agg.rawScore,
			MatchSources: sortedMatchSources(agg.matchSources),
			RepositoryID: agg.repositoryID,
			AnalysisID:   agg.analysisID,
		})
		if len(results) >= topK {
			break
		}
	}
	maxScore := 0.0
	for _, item := range results {
		if item.RawScore > maxScore {
			maxScore = item.RawScore
		}
	}
	for idx := range results {
		results[idx].DisplayScore = normalizeSearchDisplayScore(results[idx].RawScore, maxScore)
	}
	return results
}

func sortedMatchSources(items map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for kind := range items {
		out = append(out, kind)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := matchSourceOrder(out[i]), matchSourceOrder(out[j])
		if left == right {
			return out[i] < out[j]
		}
		return left < right
	})
	return out
}

func aggregateCardScore(agg *cardSearchAggregate) float64 {
	if agg == nil {
		return 0
	}
	score := agg.maxScore
	for kind, weight := range map[string]float64{
		"card":           0.35,
		"report_section": 0.18,
		"evidence":       0.10,
	} {
		kindScore := agg.topByKind[kind]
		if kindScore <= 0 {
			continue
		}
		if kindScore > agg.maxScore {
			kindScore = agg.maxScore
		}
		score += weight * kindScore
	}
	if agg.hitCount > 1 {
		score += math.Min(0.12, float64(agg.hitCount-1)*0.03) * agg.maxScore
	}
	return score
}

func normalizeSearchDisplayScore(rawScore, maxScore float64) float64 {
	if rawScore <= 0 || maxScore <= 0 {
		return 0
	}
	ratio := rawScore / maxScore
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	score := 52 + 46*math.Sqrt(ratio)
	if score > 98 {
		return 98
	}
	if score < 1 {
		return 1
	}
	return score
}

func matchSourceOrder(kind string) int {
	switch strings.TrimSpace(kind) {
	case "card":
		return 0
	case "report_section":
		return 1
	case "evidence":
		return 2
	default:
		return 10
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
