package repolib

import (
	"os"
	"strings"

	"vibe-tree/backend/internal/store"
)

// extractReportContextSummary 功能：从正式报告第一部分提取仓库级技术栈摘要。
// 参数/返回：reportText 为正式报告 markdown；返回结构化摘要与是否提取成功。
// 失败场景：缺少第一部分或关键字段全部为空时返回 false。
// 副作用：无。
func extractReportContextSummary(reportText string) (*store.RepoReportContextSummary, bool) {
	reportText = strings.TrimSpace(reportText)
	if reportText == "" {
		return nil, false
	}
	lines := strings.Split(reportText, "\n")
	headings := parseHeadings(lines)
	part1 := findHeading(headings, 2, partOneTitle)
	if part1 == nil {
		return nil, false
	}

	var nextH2 *parsedHeading
	for i := range headings {
		if headings[i].Line <= part1.Line {
			continue
		}
		if headings[i].Level == 2 {
			nextH2 = &headings[i]
			break
		}
	}
	block := extractSectionBlock(lines, headings, part1, nextH2)
	summary := &store.RepoReportContextSummary{
		GeneratedAt:         stringPtrIfNotEmpty(bulletValue(block, "生成时间")),
		StackOverview:       stringPtrIfNotEmpty(bulletValue(block, "主要语言/技术栈总览")),
		BackendSummary:      stringPtrIfNotEmpty(bulletValue(block, "后端")),
		FrontendSummary:     stringPtrIfNotEmpty(bulletValue(block, "前端")),
		OtherModulesSummary: stringPtrIfNotEmpty(bulletValue(block, "其它模块")),
	}
	if summary.GeneratedAt == nil && summary.StackOverview == nil && summary.BackendSummary == nil && summary.FrontendSummary == nil && summary.OtherModulesSummary == nil {
		return nil, false
	}
	return summary, true
}

// loadReportContextSummaryFromSnapshot 功能：从快照的 report_path 读取并派生仓库级摘要。
// 参数/返回：snapshot 提供 report_path；返回摘要与是否成功。
// 失败场景：报告路径缺失、文件不存在或解析失败时返回 false。
// 副作用：读取磁盘文件。
func loadReportContextSummaryFromSnapshot(snapshot store.RepoSnapshot) (*store.RepoReportContextSummary, bool) {
	if snapshot.ReportPath == nil || strings.TrimSpace(*snapshot.ReportPath) == "" {
		return nil, false
	}
	body, err := os.ReadFile(strings.TrimSpace(*snapshot.ReportPath))
	if err != nil {
		return nil, false
	}
	return extractReportContextSummary(string(body))
}

func enrichSnapshotsWithReportContext(snapshots []store.RepoSnapshot) []store.RepoSnapshot {
	if len(snapshots) == 0 {
		return snapshots
	}
	enriched := make([]store.RepoSnapshot, len(snapshots))
	for i, snapshot := range snapshots {
		enriched[i] = snapshot
		if summary, ok := loadReportContextSummaryFromSnapshot(snapshot); ok {
			enriched[i].ReportContext = summary
		}
	}
	return enriched
}
