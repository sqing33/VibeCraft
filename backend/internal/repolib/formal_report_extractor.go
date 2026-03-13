package repolib

import (
	"regexp"
	"sort"
	"strings"

	"vibecraft/backend/internal/store"
)

var evidenceLinePattern = regexp.MustCompile(
	"`?([A-Za-z0-9_./-]+\\.[A-Za-z0-9_+.-]+):(\\d+)(?:-(\\d+))?`?" +
		"(?:\\s+\\[([^\\]]+)\\])?(?:\\s+-\\s+`?(.+?)`?)?$",
)

type extractedEvidence struct {
	Path      string
	Line      *int64
	Dimension *string
	Snippet   *string
}

// extractFormalReportCardsV2 功能：从正式报告（v2 结构）抽取 cards/evidence 输入。
// 参数/返回：reportText 为已验证的正式报告文本；features 为分析请求 features（用于问题标题对齐）；返回 cards 输入。
func extractFormalReportCardsV2(reportText string, features []string) ([]store.RepoKnowledgeCardInput, int, int) {
	lines := strings.Split(reportText, "\n")
	headings := parseHeadings(lines)

	part3 := findHeading(headings, 2, partThreeTitle)
	part4 := findHeading(headings, 2, partFourTitle)
	part5 := findHeading(headings, 2, partFiveTitle)

	cards := []store.RepoKnowledgeCardInput{}
	totalEvidence := 0

	// Evidence blocks keyed by "characteristic:{i}" and "question:{i}" are stored under part5 as H3 headings.
	evidenceBlocks := map[string][]string{}
	if part5 != nil {
		h3 := collectAnyH3(headings, part5.Line, len(lines))
		for _, h := range h3 {
			title := strings.TrimSpace(h.Title)
			if m := formalCharacteristicPattern.FindStringSubmatch(title); len(m) >= 2 {
				key := "characteristic:" + m[1]
				evidenceBlocks[key] = extractSectionBlock(lines, headings, &h, nextAnyH3Boundary(h3, h.Line, len(lines)))
				continue
			}
			if m := formalQuestionPattern.FindStringSubmatch(title); len(m) >= 2 {
				key := "question:" + m[1]
				evidenceBlocks[key] = extractSectionBlock(lines, headings, &h, nextAnyH3Boundary(h3, h.Line, len(lines)))
				continue
			}
		}
	}

	// Characteristics -> project_characteristic cards.
	charHeadings := collectH3ByPattern(headings, part3, part4, formalCharacteristicPattern)
	for idx, h := range charHeadings {
		m := formalCharacteristicPattern.FindStringSubmatch(h.Title)
		if len(m) < 3 {
			continue
		}
		title := strings.TrimSpace(m[2])
		block := extractSectionBlock(lines, headings, &h, nextH3Boundary(charHeadings, h.Line, part4Line(part4, len(lines))))
		fields := parseSimpleBullets(block)
		conf := normalizeConfidence(fields["置信度"])
		summaryCandidate := firstNonEmpty(fields["目标"], fields["动机"])
		conclusion := firstNonEmpty(firstNonEmpty(summaryCandidate, shortReportSummary(fields["思路"])), title)
		mechanism := joinNonEmpty("\n", "思路: "+fields["思路"], "取舍: "+fields["取舍"])
		summary := supportingSummary(summaryCandidate, conclusion, mechanism)
		sectionTitle := h.Title

		card := store.RepoKnowledgeCardInput{
			Title:        title,
			CardType:     "project_characteristic",
			Conclusion:   stringPtrIfNotEmpty(conclusion),
			Summary:      summary,
			Mechanism:    stringPtrIfNotEmpty(mechanism),
			Confidence:   stringPtrIfNotEmpty(conf),
			SectionTitle: stringPtrIfNotEmpty(sectionTitle),
			SortIndex:    idx + 1,
		}
		key := "characteristic:" + m[1]
		evidence := parseEvidenceLines(evidenceBlocks[key], "characteristic")
		card.Evidence = toEvidenceInputs(evidence)
		totalEvidence += len(card.Evidence)
		if hasRenderableCardContent(card) {
			cards = append(cards, card)
		}
	}

	// Questions -> feature_pattern cards.
	normalizedFeatures := []string{}
	for _, f := range features {
		if t := normalizeReportQuestionTitle(f); t != "" {
			normalizedFeatures = append(normalizedFeatures, t)
		}
	}
	qHeadings := collectH3ByPattern(headings, part4, part5, formalQuestionPattern)
	for idx, h := range qHeadings {
		m := formalQuestionPattern.FindStringSubmatch(h.Title)
		if len(m) < 3 {
			continue
		}
		qTitle := strings.TrimSpace(m[2])
		block := extractSectionBlock(lines, headings, &h, nextH3Boundary(qHeadings, h.Line, part5Line(part5, len(lines))))
		fields := parseSimpleBullets(block)
		conf := normalizeConfidence(fields["置信度"])
		conclusion := firstNonEmpty(fields["结论"], qTitle)
		mechanism := joinNonEmpty("\n", "思路: "+fields["思路"], "取舍: "+fields["取舍"])
		summaryCandidate := firstNonEmpty(shortReportSummary(fields["思路"]), shortReportSummary(fields["取舍"]))
		summary := supportingSummary(summaryCandidate, conclusion, mechanism)

		// Prefer requested feature title when aligned.
		title := qTitle
		if idx < len(normalizedFeatures) && normalizedFeatures[idx] != "" {
			title = normalizedFeatures[idx]
		}
		sectionTitle := h.Title

		card := store.RepoKnowledgeCardInput{
			Title:        title,
			CardType:     "feature_pattern",
			Conclusion:   stringPtrIfNotEmpty(conclusion),
			Summary:      summary,
			Mechanism:    stringPtrIfNotEmpty(mechanism),
			Confidence:   stringPtrIfNotEmpty(conf),
			SectionTitle: stringPtrIfNotEmpty(sectionTitle),
			SortIndex:    len(cards) + 1,
		}
		key := "question:" + m[1]
		evidence := parseEvidenceLines(evidenceBlocks[key], "question")
		card.Evidence = toEvidenceInputs(evidence)
		totalEvidence += len(card.Evidence)
		if hasRenderableCardContent(card) {
			cards = append(cards, card)
		}
	}

	return cards, len(cards), totalEvidence
}

func parseSimpleBullets(lines []string) map[string]string {
	out := map[string]string{}
	for _, raw := range lines {
		stripped := strings.TrimSpace(raw)
		if !strings.HasPrefix(stripped, "- ") {
			continue
		}
		body := strings.TrimSpace(stripped[2:])
		parts := strings.FieldsFunc(body, func(r rune) bool { return r == ':' || r == '：' })
		if len(parts) < 2 {
			continue
		}
		label := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(body[len(parts[0])+1:])
		if label != "" && value != "" {
			out[label] = value
		}
	}
	return out
}

func normalizeConfidence(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	if _, ok := validConfidence[value]; ok {
		return value
	}
	return ""
}

func joinNonEmpty(sep string, parts ...string) string {
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasSuffix(p, ":") {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, sep)
}

func shortReportSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, sep := range []string{"\n", "。", "；", ";"} {
		if idx := strings.Index(raw, sep); idx > 0 {
			raw = raw[:idx]
			break
		}
	}
	raw = strings.TrimSpace(raw)
	if len([]rune(raw)) > 72 {
		return excerpt(raw, 72)
	}
	return raw
}

func supportingSummary(summary, conclusion, mechanism string) string {
	summary = normalizeCardText(summary)
	if summary == "" {
		return ""
	}
	conclusion = normalizeCardText(conclusion)
	if conclusion != "" && summary == conclusion {
		return ""
	}
	mechanism = normalizeCardText(mechanism)
	summaryPrefix := strings.TrimSuffix(strings.TrimSuffix(summary, "..."), "…")
	if summaryPrefix != "" && mechanism != "" && strings.HasPrefix(mechanism, summaryPrefix) {
		return ""
	}
	return summary
}

func normalizeCardText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimPrefix(raw, "思路: ")
	raw = strings.TrimPrefix(raw, "思路：")
	return strings.Join(strings.Fields(raw), " ")
}

func hasRenderableCardContent(card store.RepoKnowledgeCardInput) bool {
	if strings.TrimSpace(card.Title) == "" || strings.TrimSpace(card.CardType) == "" {
		return false
	}
	return strings.TrimSpace(card.Summary) != "" || strings.TrimSpace(pointerValue(card.Conclusion)) != ""
}

func parseEvidenceLines(lines []string, defaultLabel string) []extractedEvidence {
	seen := map[string]extractedEvidence{}
	for _, raw := range lines {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "-"))
		if text == "" {
			continue
		}
		match := evidenceLinePattern.FindStringSubmatch(raw)
		if len(match) < 3 {
			continue
		}
		path := strings.TrimSpace(match[1])
		lineNum := int64(parseInt(match[2]))
		label := strings.TrimSpace(match[4])
		if label == "" {
			label = defaultLabel
		}
		snippet := strings.TrimSpace(match[5])
		var snippetPtr *string
		if snippet != "" {
			snippetPtr = stringPtrIfNotEmpty(snippet)
		}
		dimPtr := stringPtrIfNotEmpty(label)
		linePtr := &lineNum
		key := strings.Join([]string{path, match[2], label}, "|")
		prev, ok := seen[key]
		if !ok || (prev.Snippet == nil && snippetPtr != nil) || (prev.Snippet != nil && snippetPtr != nil && len(*snippetPtr) > len(*prev.Snippet)) {
			seen[key] = extractedEvidence{Path: path, Line: linePtr, Dimension: dimPtr, Snippet: snippetPtr}
		}
	}
	out := make([]extractedEvidence, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			li := int64(0)
			lj := int64(0)
			if out[i].Line != nil {
				li = *out[i].Line
			}
			if out[j].Line != nil {
				lj = *out[j].Line
			}
			return li < lj
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func toEvidenceInputs(items []extractedEvidence) []store.RepoKnowledgeEvidenceInput {
	out := make([]store.RepoKnowledgeEvidenceInput, 0, len(items))
	for idx, item := range items {
		out = append(out, store.RepoKnowledgeEvidenceInput{
			Path:      item.Path,
			Line:      item.Line,
			Snippet:   item.Snippet,
			Dimension: item.Dimension,
			SortIndex: idx + 1,
		})
	}
	return out
}

func collectAnyH3(headings []parsedHeading, startLine, endLine int) []parsedHeading {
	out := []parsedHeading{}
	for _, h := range headings {
		if h.Level == 3 && h.Line > startLine && h.Line < endLine {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

func nextAnyH3Boundary(h3 []parsedHeading, currentLine int, fallback int) *parsedHeading {
	for _, h := range h3 {
		if h.Line > currentLine {
			cp := h
			return &cp
		}
	}
	return &parsedHeading{Line: fallback}
}
