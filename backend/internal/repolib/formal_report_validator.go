package repolib

import (
	"regexp"
	"sort"
	"strings"
)

var (
	formalHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	formalFileRefPattern = regexp.MustCompile("`?([A-Za-z0-9_./-]+\\.[A-Za-z0-9_+.-]+):(\\d+)(?:-(\\d+))?`?")

	formalCharacteristicPattern = regexp.MustCompile(`^特点\s+(\d+)\s*[:：]\s*(.+?)\s*$`)
	formalQuestionPattern       = regexp.MustCompile(`^问题\s+(\d+)\s*[:：]\s*(.+?)\s*$`)
)

const (
	formalH1Title    = "# GitHub 功能实现原理报告"
	partOneTitle     = "第一部分：技术栈与模块语言"
	partTwoTitle     = "第二部分：项目用途与核心特点"
	partThreeTitle   = "第三部分：特点实现思路"
	partFourTitle    = "第四部分：提问与解答"
	partFiveTitle    = "第五部分：实现定位与证据"
	riskSectionTitle = "风险与局限"
)

var partOneRequiredBullets = []string{
	"仓库",
	"请求 Ref",
	"解析 Ref",
	"Commit",
	"生成时间",
	"主要语言/技术栈总览",
	"后端",
	"前端",
	"其它模块",
}

var partTwoRequiredBullets = []string{
	"项目做什么用",
	"典型使用场景",
	"核心特点概览",
}

var characteristicRequiredBullets = []string{
	"动机",
	"目标",
	"思路",
	"取舍",
	"置信度",
}

var questionRequiredBullets = []string{
	"结论",
	"思路",
	"取舍",
	"置信度",
}

var validConfidence = map[string]struct{}{
	"high":   {},
	"medium": {},
	"low":    {},
}

type parsedHeading struct {
	Level int
	Title string
	Line  int
}

// validateFormalReportV2 功能：校验 Repo Library 正式报告（v2 格式），并输出 fatal/warning 结果。
// 参数/返回：reportText 为候选报告正文；features 为请求 features 列表；返回 fatalErrors, warnings 与统计信息。
func validateFormalReportV2(reportText string, features []string) (fatalErrors []string, warnings []string, stats reportValidationResult) {
	stats = reportValidationResult{
		Status:               "ok",
		Command:              "validate-report",
		Valid:                false,
		Errors:               nil,
		Warnings:             nil,
		FeatureCountExpected: len(uniqueTrimmed(features)),
		FeatureCountFound:    0,
		TableCount:           0,
	}
	if strings.Contains(reportText, "```") {
		fatalErrors = append(fatalErrors, "正式报告正文禁止包含 Markdown 代码围栏（```）。")
	}
	lines := strings.Split(reportText, "\n")
	headings := parseHeadings(lines)
	if len(headings) == 0 {
		fatalErrors = append(fatalErrors, "缺少 Markdown 标题结构。")
		return
	}

	firstLine := strings.TrimSpace(strings.TrimPrefix(lines[0], "\ufeff"))
	if firstLine != strings.TrimPrefix(formalH1Title, "# ") && strings.TrimSpace(lines[0]) != formalH1Title {
		// Historical reports may include BOM; require exact H1 line.
		fatalErrors = append(fatalErrors, "第一行必须直接是 `# GitHub 功能实现原理报告`。")
	}

	requiredH2 := []string{"Run 1", partOneTitle, partTwoTitle, partThreeTitle, partFourTitle, partFiveTitle}
	pos := map[string]int{}
	for _, title := range requiredH2 {
		h := findHeading(headings, 2, title)
		if h == nil {
			fatalErrors = append(fatalErrors, "缺少 `## "+title+"` 标题。")
			continue
		}
		pos[title] = h.Line
	}
	if len(pos) == len(requiredH2) {
		order := make([]int, 0, len(requiredH2))
		for _, title := range requiredH2 {
			order = append(order, pos[title])
		}
		sorted := append([]int(nil), order...)
		sort.Ints(sorted)
		for i := range order {
			if order[i] != sorted[i] {
				fatalErrors = append(fatalErrors, "核心 H2 标题顺序不正确。")
				break
			}
		}
	}

	part1 := findHeading(headings, 2, partOneTitle)
	part2 := findHeading(headings, 2, partTwoTitle)
	part3 := findHeading(headings, 2, partThreeTitle)
	part4 := findHeading(headings, 2, partFourTitle)
	part5 := findHeading(headings, 2, partFiveTitle)

	part1Block := extractSectionBlock(lines, headings, part1, part2)
	part2Block := extractSectionBlock(lines, headings, part2, part3)
	part3Block := extractSectionBlock(lines, headings, part3, part4)
	part4Block := extractSectionBlock(lines, headings, part4, part5)
	part5Block := extractSectionBlock(lines, headings, part5, nil)

	for _, label := range partOneRequiredBullets {
		if part1 != nil && bulletValue(part1Block, label) == "" {
			fatalErrors = append(fatalErrors, "`## "+partOneTitle+"` 缺少 `- "+label+":` 字段。")
		}
	}
	for _, label := range partTwoRequiredBullets {
		if part2 != nil && bulletValue(part2Block, label) == "" {
			fatalErrors = append(fatalErrors, "`## "+partTwoTitle+"` 缺少 `- "+label+":` 字段。")
		}
	}

	if part2 != nil {
		if !hasH3Section(lines, headings, part2.Line, part3Line(part3, len(lines)), riskSectionTitle) {
			fatalErrors = append(fatalErrors, "`## "+partTwoTitle+"` 缺少 `### "+riskSectionTitle+"` 小节。")
		}
	}

	// file:line refs are only allowed in part 5.
	for _, block := range [][]string{part1Block, part2Block, part3Block, part4Block} {
		if len(extractFileRefs(block)) > 0 {
			fatalErrors = append(fatalErrors, "第一到第四部分禁止出现 `file:line` 引用。")
			break
		}
	}

	// Part 3 characteristics.
	charHeadings := collectH3ByPattern(headings, part3, part4, formalCharacteristicPattern)
	if part3 != nil && part4 != nil && len(charHeadings) == 0 {
		fatalErrors = append(fatalErrors, "`## "+partThreeTitle+"` 至少需要 1 个 `### 特点 1: ...`。")
	}
	charIndices := []int{}
	for _, h := range charHeadings {
		m := formalCharacteristicPattern.FindStringSubmatch(h.Title)
		if len(m) < 3 {
			continue
		}
		idx := parseInt(m[1])
		title := strings.TrimSpace(m[2])
		block := extractSectionBlock(lines, headings, &h, nextH3Boundary(charHeadings, h.Line, part4Line(part4, len(lines))))
		for _, label := range characteristicRequiredBullets {
			if bulletValue(block, label) == "" {
				fatalErrors = append(fatalErrors, "`特点 "+m[1]+": "+title+"` 缺少 `- "+label+":` 字段。")
			}
		}
		conf := strings.ToLower(strings.TrimSpace(bulletValue(block, "置信度")))
		if _, ok := validConfidence[conf]; !ok {
			fatalErrors = append(fatalErrors, "`特点 "+m[1]+": "+title+"` 的 `- 置信度:` 必须是 high|medium|low。")
		}
		charIndices = append(charIndices, idx)
	}
	if len(charIndices) > 0 {
		sort.Ints(charIndices)
		for i := 0; i < len(charIndices); i++ {
			if charIndices[i] != i+1 {
				fatalErrors = append(fatalErrors, "`## "+partThreeTitle+"` 的特点编号必须从 1 开始连续递增。")
				break
			}
		}
	}

	// Part 4 questions.
	normalizedFeatures := make([]string, 0, len(features))
	for _, f := range features {
		title := normalizeReportQuestionTitle(f)
		if title != "" {
			normalizedFeatures = append(normalizedFeatures, title)
		}
	}
	qHeadings := collectH3ByPattern(headings, part4, part5, formalQuestionPattern)
	stats.FeatureCountFound = len(qHeadings)
	if part4 != nil && part5 != nil && len(qHeadings) != len(normalizedFeatures) {
		fatalErrors = append(fatalErrors, "`## "+partFourTitle+"` 问题数量不匹配。")
	}
	qIndices := []int{}
	for idx, h := range qHeadings {
		m := formalQuestionPattern.FindStringSubmatch(h.Title)
		if len(m) < 3 {
			continue
		}
		qIndex := parseInt(m[1])
		qTitle := strings.TrimSpace(m[2])
		block := extractSectionBlock(lines, headings, &h, nextH3Boundary(qHeadings, h.Line, part5Line(part5, len(lines))))
		for _, label := range questionRequiredBullets {
			if bulletValue(block, label) == "" {
				fatalErrors = append(fatalErrors, "`问题 "+m[1]+": "+qTitle+"` 缺少 `- "+label+":` 字段。")
			}
		}
		conf := strings.ToLower(strings.TrimSpace(bulletValue(block, "置信度")))
		if _, ok := validConfidence[conf]; !ok {
			fatalErrors = append(fatalErrors, "`问题 "+m[1]+": "+qTitle+"` 的 `- 置信度:` 必须是 high|medium|low。")
		}
		qIndices = append(qIndices, qIndex)
		// Best-effort title matching; mismatch is a warning to avoid brittleness.
		if idx < len(normalizedFeatures) && normalizedFeatures[idx] != "" {
			if normalizeReportQuestionTitle(qTitle) != normalizedFeatures[idx] {
				warnings = append(warnings, "问题标题与请求 features 不一致（请确保问题标题能对应到请求项）。")
			}
		}
	}
	if len(qIndices) > 0 {
		sort.Ints(qIndices)
		for i := 0; i < len(qIndices); i++ {
			if qIndices[i] != i+1 {
				fatalErrors = append(fatalErrors, "`## "+partFourTitle+"` 的问题编号必须从 1 开始连续递增。")
				break
			}
		}
	}

	// Part 5 must include at least one file ref.
	if part5 != nil {
		if refs := extractFileRefs(part5Block); len(refs) == 0 {
			fatalErrors = append(fatalErrors, "`## "+partFiveTitle+"` 必须包含至少 1 处 `file:line` 引用。")
		}
	}

	// Extractability precheck: require at least one characteristic or question.
	if len(charHeadings) == 0 && len(qHeadings) == 0 {
		fatalErrors = append(fatalErrors, "报告缺少可抽取的特点/问题结构，无法生成知识卡片。")
	}

	if len(fatalErrors) == 0 {
		stats.Valid = true
	}
	stats.Errors = fatalErrors
	stats.Warnings = warnings
	return
}

func parseHeadings(lines []string) []parsedHeading {
	out := []parsedHeading{}
	for idx, line := range lines {
		match := formalHeadingPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 3 {
			continue
		}
		out = append(out, parsedHeading{
			Level: len(match[1]),
			Title: strings.TrimSpace(match[2]),
			Line:  idx,
		})
	}
	return out
}

func findHeading(headings []parsedHeading, level int, title string) *parsedHeading {
	for _, h := range headings {
		if h.Level == level && h.Title == title {
			cp := h
			return &cp
		}
	}
	return nil
}

func extractSectionBlock(lines []string, headings []parsedHeading, start *parsedHeading, end *parsedHeading) []string {
	if start == nil {
		return nil
	}
	boundary := len(lines)
	if end != nil {
		boundary = end.Line
	}
	return lines[start.Line+1 : boundary]
}

func bulletValue(block []string, label string) string {
	for _, line := range block {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "- ") {
			continue
		}
		body := strings.TrimSpace(stripped[2:])
		if strings.HasPrefix(body, label+":") {
			return strings.TrimSpace(strings.TrimPrefix(body, label+":"))
		}
		if strings.HasPrefix(body, label+"：") {
			return strings.TrimSpace(strings.TrimPrefix(body, label+"："))
		}
	}
	return ""
}

func extractFileRefs(block []string) []string {
	refs := []string{}
	for _, line := range block {
		matches := formalFileRefPattern.FindAllString(line, -1)
		if len(matches) > 0 {
			refs = append(refs, matches...)
		}
	}
	return refs
}

func hasH3Section(lines []string, headings []parsedHeading, startLine, endLine int, title string) bool {
	for _, h := range headings {
		if h.Level == 3 && h.Title == title && h.Line > startLine && h.Line < endLine {
			return true
		}
	}
	return false
}

func part3Line(h *parsedHeading, fallback int) int {
	if h == nil {
		return fallback
	}
	return h.Line
}

func part4Line(h *parsedHeading, fallback int) int {
	if h == nil {
		return fallback
	}
	return h.Line
}

func part5Line(h *parsedHeading, fallback int) int {
	if h == nil {
		return fallback
	}
	return h.Line
}

func collectH3ByPattern(headings []parsedHeading, start *parsedHeading, end *parsedHeading, pattern *regexp.Regexp) []parsedHeading {
	if start == nil || end == nil {
		return nil
	}
	out := []parsedHeading{}
	for _, h := range headings {
		if h.Level == 3 && h.Line > start.Line && h.Line < end.Line && pattern.MatchString(h.Title) {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

func nextH3Boundary(h3 []parsedHeading, currentLine int, fallback int) *parsedHeading {
	for _, h := range h3 {
		if h.Line > currentLine {
			cp := h
			return &cp
		}
	}
	return &parsedHeading{Line: fallback}
}

func parseInt(raw string) int {
	raw = strings.TrimSpace(raw)
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
