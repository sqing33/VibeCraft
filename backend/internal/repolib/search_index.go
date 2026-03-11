package repolib

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vibe-tree/backend/internal/logx"
	"vibe-tree/backend/internal/paths"
	"vibe-tree/backend/internal/repolib/searchdb"
	"vibe-tree/backend/internal/store"
)

type searchIndex struct {
	sdb     *searchdb.Service
	closers []interface{ Close() error }
}

func openSearchIndex(ctx context.Context) (*searchIndex, error) {
	repoLibraryDir, err := paths.RepoLibraryDir()
	if err != nil {
		return nil, err
	}
	searchDir := filepath.Join(repoLibraryDir, "search")
	if err := paths.EnsureDir(searchDir); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(searchDir, "search.db")

	vecPath := strings.TrimSpace(os.Getenv("VIBE_TREE_SQLITE_VEC_PATH")) // optional override
	embedderMode := strings.ToLower(strings.TrimSpace(os.Getenv("VIBE_TREE_EMBEDDER")))
	if embedderMode == "" {
		embedderMode = "hugot"
	}

	// Default: local embedder (auto-download). Fallback: keyword-only.
	var embedder searchdb.Embedder = &missingEmbedder{}
	closers := []interface{ Close() error }{}
	switch embedderMode {
	case "missing", "none", "off", "disabled":
		embedder = &missingEmbedder{}
	case "hugot", "local":
		modelsDir := filepath.Join(repoLibraryDir, "models", "embedder")
		e, err := newHugotEmbedder(ctx, modelsDir)
		if err != nil {
			logx.Warn("repolib", "embedder_init", "local embedder init failed; falling back to keyword-only", "err", err.Error())
			embedder = &missingEmbedder{}
		} else {
			embedder = e
			closers = append(closers, e)
		}
	default:
		logx.Warn("repolib", "embedder_init", "unknown VIBE_TREE_EMBEDDER; falling back to keyword-only", "value", embedderMode)
		embedder = &missingEmbedder{}
	}

	sdb, err := searchdb.Open(ctx, searchdb.OpenParams{DBPath: dbPath, Embedder: embedder, VecExtension: vecPath})
	if err != nil {
		return nil, err
	}
	return &searchIndex{sdb: sdb, closers: closers}, nil
}

func (idx *searchIndex) Close() error {
	if idx == nil || idx.sdb == nil {
		return nil
	}
	for _, c := range idx.closers {
		_ = c.Close()
	}
	return idx.sdb.Close()
}

func (s *Service) ensureSearchIndex(ctx context.Context) (*searchIndex, error) {
	if s == nil {
		return nil, fmt.Errorf("repo library service not configured")
	}
	if s.searchIdx != nil {
		return s.searchIdx, nil
	}
	idx, err := openSearchIndex(ctx)
	if err != nil {
		return nil, err
	}
	s.searchIdx = idx
	return idx, nil
}

func (s *Service) refreshSearchIndexForSnapshot(ctx context.Context, source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun) (map[string]any, error) {
	idx, err := s.ensureSearchIndex(ctx)
	if err != nil {
		return nil, err
	}
	reportPath := strings.TrimSpace(pointerValue(snapshot.ReportPath))
	if reportPath == "" {
		reportPath = filepath.Join(snapshot.StoragePath, "report.md")
	}
	reportText, _ := os.ReadFile(reportPath)

	cards, err := s.store.ListRepoCards(ctx, store.ListRepoCardsParams{
		RepoSourceID:   source.ID,
		RepoSnapshotID: snapshot.ID,
		AnalysisRunID:  run.ID,
		Limit:          1000,
	})
	if err != nil {
		return nil, err
	}
	evidenceByCard := map[string][]store.RepoKnowledgeEvidence{}
	for _, card := range cards {
		ev, err := s.store.ListRepoEvidenceByCard(ctx, card.ID)
		if err != nil {
			continue
		}
		evidenceByCard[card.ID] = ev
	}

	chunks := buildSearchChunks(source, snapshot, run, string(reportText), cards, evidenceByCard)
	if err := idx.sdb.DeleteSnapshot(ctx, snapshot.ID); err != nil {
		return nil, err
	}
	if err := idx.sdb.UpsertChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":        "ok",
		"engine":        "go-searchdb",
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"vec_enabled":   idx.sdb.VecEnabled(),
		"chunk_count":   len(chunks),
		"snapshot_id":   snapshot.ID,
		"repository_id": source.ID,
	}, nil
}

func buildSearchChunks(
	source store.RepoSource,
	snapshot store.RepoSnapshot,
	run store.RepoAnalysisRun,
	reportText string,
	cards []store.RepoKnowledgeCard,
	evidenceByCard map[string][]store.RepoKnowledgeEvidence,
) []searchdb.Chunk {
	now := time.Now().UnixMilli()
	chunks := []searchdb.Chunk{}
	cardBySectionTitle := map[string]store.RepoKnowledgeCard{}
	for _, card := range cards {
		sectionTitle := strings.TrimSpace(pointerValue(card.SectionTitle))
		if sectionTitle != "" {
			cardBySectionTitle[sectionTitle] = card
		}
	}

	// report_section chunks
	lines := strings.Split(reportText, "\n")
	headings := parseHeadings(lines)
	part3 := findHeading(headings, 2, partThreeTitle)
	part4 := findHeading(headings, 2, partFourTitle)
	part5 := findHeading(headings, 2, partFiveTitle)
	part3Start, part3End := sectionBounds(part3, part4, len(lines))
	part4Start, part4End := sectionBounds(part4, part5, len(lines))
	for _, h := range headings {
		// Only index feature/question level sections. Top-level report sections
		// such as "第三部分/第四部分/第五部分" are too broad and tend to drown out
		// answer-shaped card results during retrieval.
		if h.Level < 3 || h.Level > 4 {
			continue
		}
		if !headingWithin(h.Line, part3Start, part3End) && !headingWithin(h.Line, part4Start, part4End) {
			continue
		}
		titlePath := reportHeadingPath(headings, h)
		block := extractSectionBlock(lines, headings, &h, nextHeadingAtOrAbove(headings, h.Line, h.Level, len(lines)))
		text := strings.TrimSpace(strings.Join(block, "\n"))
		if text == "" {
			continue
		}
		card, ok := cardBySectionTitle[strings.TrimSpace(h.Title)]
		if !ok {
			// Report sections without a corresponding card are not answer-shaped
			// enough for user-facing retrieval.
			continue
		}
		chunkID := fmt.Sprintf("report:%s:%s", snapshot.ID, sha256Short(titlePath))
		display := strings.TrimSpace(titlePath + "\n\n" + text)
		search := strings.TrimSpace(titlePath + "\n\n" + text)
		chunks = append(chunks, searchdb.Chunk{
			ChunkID:        chunkID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			SourceKind:     "report_section",
			SourceRefID:    card.ID,
			Title:          titlePath,
			DisplayText:    display,
			SearchText:     search,
			EvidenceRefs:   strings.Join(extractFileRefs(block), "\n"),
			TextExcerpt:    excerpt(display, 360),
			ContentHash:    sha256Hex(search),
			UpdatedAt:      now,
		})
	}

	// card + evidence chunks
	for _, card := range cards {
		ev := evidenceByCard[card.ID]
		refs := flattenEvidenceRefs(ev)
		tags := strings.Join(card.Tags, " ")
		conclusion := strings.TrimSpace(pointerValue(card.Conclusion))
		display := strings.TrimSpace(joinNonEmpty("\n\n", card.Title, conclusion, card.Summary, pointerValue(card.Mechanism)))
		search := strings.TrimSpace(strings.Join([]string{
			"Title: " + card.Title,
			"Conclusion: " + conclusion,
			"Summary: " + card.Summary,
			"Mechanism: " + pointerValue(card.Mechanism),
			"Tags: " + tags,
			"EvidenceRefs: " + strings.Join(refs, " "),
		}, "\n"))
		chunks = append(chunks, searchdb.Chunk{
			ChunkID:        "card:" + card.ID,
			RepoSourceID:   source.ID,
			RepoSnapshotID: snapshot.ID,
			AnalysisRunID:  run.ID,
			SourceKind:     "card",
			SourceRefID:    card.ID,
			Title:          card.Title,
			DisplayText:    display,
			SearchText:     search,
			TagsFlat:       tags,
			EvidenceRefs:   strings.Join(refs, "\n"),
			TextExcerpt:    excerpt(display, 360),
			ContentHash:    sha256Hex(search),
			UpdatedAt:      now,
		})
		for _, item := range ev {
			lineText := ""
			if item.Line != nil {
				lineText = fmt.Sprintf("%d", *item.Line)
			}
			evTitle := fmt.Sprintf("%s:%s", item.Path, lineText)
			evDisplay := strings.TrimSpace(evTitle + "\n" + pointerValue(item.Snippet))
			evSearch := strings.TrimSpace(strings.Join([]string{
				"Path: " + item.Path,
				"Line: " + lineText,
				"Dimension: " + pointerValue(item.Dimension),
				"Snippet: " + pointerValue(item.Snippet),
				"CardTitle: " + card.Title,
				"CardConclusion: " + conclusion,
			}, "\n"))
			chunks = append(chunks, searchdb.Chunk{
				ChunkID:        "evidence:" + item.ID,
				RepoSourceID:   source.ID,
				RepoSnapshotID: snapshot.ID,
				AnalysisRunID:  run.ID,
				SourceKind:     "evidence",
				SourceRefID:    card.ID,
				Title:          evTitle,
				DisplayText:    evDisplay,
				SearchText:     evSearch,
				SymbolsFlat:    "",
				EvidenceRefs:   strings.Join([]string{evTitle}, "\n"),
				TextExcerpt:    excerpt(evDisplay, 260),
				ContentHash:    sha256Hex(evSearch),
				UpdatedAt:      now,
			})
		}
	}

	return chunks
}

func sectionBounds(start, next *parsedHeading, fallback int) (int, int) {
	if start == nil {
		return 0, 0
	}
	end := fallback
	if next != nil {
		end = next.Line
	}
	return start.Line, end
}

func headingWithin(line, start, end int) bool {
	if start == 0 {
		return false
	}
	return line > start && line < end
}

func sha256Short(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])[:16]
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func excerpt(raw string, maxChars int) string {
	text := strings.TrimSpace(strings.Join(strings.Fields(raw), " "))
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

func flattenEvidenceRefs(items []store.RepoKnowledgeEvidence) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Path == "" || item.Line == nil {
			continue
		}
		ref := fmt.Sprintf("%s:%d", item.Path, *item.Line)
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

func nextHeadingAtOrAbove(headings []parsedHeading, startLine int, level int, fallback int) *parsedHeading {
	for _, h := range headings {
		if h.Line > startLine && h.Level <= level {
			cp := h
			return &cp
		}
	}
	return &parsedHeading{Line: fallback}
}

func reportHeadingPath(headings []parsedHeading, h parsedHeading) string {
	parts := []string{h.Title}
	parentLevel := h.Level - 1
	for parentLevel >= 2 {
		for i := len(headings) - 1; i >= 0; i-- {
			if headings[i].Line < h.Line && headings[i].Level == parentLevel {
				parts = append([]string{headings[i].Title}, parts...)
				h = headings[i]
				break
			}
		}
		parentLevel--
	}
	return strings.Join(parts, " > ")
}
