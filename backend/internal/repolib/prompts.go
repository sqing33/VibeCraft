package repolib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"vibe-tree/backend/internal/store"
)

type analysisPrepareResult struct {
	ResolvedRef   string
	CommitSHA     string
	CodeIndexPath string
	SnapshotDir   string
	SourceDir     string
	ReportPath    string
}

func buildPlanningTurnPrompt(source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, prepared analysisPrepareResult) string {
	features := strings.Join(run.Features, "；")
	return strings.TrimSpace(fmt.Sprintf(`你正在为 vibe-tree 的 Repo Library 执行一次真实仓库分析。你当前工作目录就是目标仓库源码目录。

分析上下文：
- 仓库：%s
- owner/repo：%s/%s
- 请求 Ref：%s
- 解析 Ref：%s
- Commit：%s
- 分析深度：%s
- 输出语言：%s
- 关注点：%s
- 代码索引：%s

任务要求：
1. 先阅读 README、入口文件、主要目录和与关注点相关的核心实现。
2. 先不要输出最终报告。
3. 请输出一份“分析计划与证据采集方案”，内容至少包括：
   - 你准备重点检查的入口 / 模块 / 文件
   - 你预期的实现主链路
   - 哪些点最可能缺证据或需要标记为 inference
   - 下一步你将如何产出最终报告
4. 只输出中文 Markdown，不要输出 JSON。
5. 不要编造证据；如果还没验证，只能写待验证项。`, source.RepoURL, source.Owner, source.Repo, snapshot.RequestedRef, prepared.ResolvedRef, prepared.CommitSHA, run.Depth, run.Language, features, prepared.CodeIndexPath))
}

type reportFeaturePrompt struct {
	Index         int
	Title         string
	RequiresTable bool
}

func buildFinalReportTurnPrompt(source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, prepared analysisPrepareResult) string {
	features := buildReportFeaturePrompts(run.Features)
	return buildFormalReportPrompt(source, snapshot, run, prepared, features, nil, 0, 0)
}

func buildReportRepairTurnPrompt(source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, prepared analysisPrepareResult, errors []string, attempt, maxRetry int) string {
	features := buildReportFeaturePrompts(run.Features)
	return buildFormalReportPrompt(source, snapshot, run, prepared, features, errors, attempt, maxRetry)
}

var leadingNumberPrefix = regexp.MustCompile(`^\s*\d+\s*[\.、:：)）]\s*|^\s*\d+\s+`)

func normalizeReportQuestionTitle(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = leadingNumberPrefix.ReplaceAllString(trimmed, "")
	return strings.TrimSpace(trimmed)
}

func buildReportFeaturePrompts(features []string) []reportFeaturePrompt {
	out := make([]reportFeaturePrompt, 0, len(features))
	for index, raw := range features {
		title := normalizeReportQuestionTitle(raw)
		if title == "" {
			continue
		}
		out = append(out, reportFeaturePrompt{
			Index:         len(out) + 1,
			Title:         title,
			RequiresTable: featureRequiresTable(title),
		})
		_ = index
	}
	if len(out) == 0 {
		out = append(out, reportFeaturePrompt{Index: 1, Title: "你想了解这个项目的哪些实现思路", RequiresTable: false})
	}
	return out
}

func featureRequiresTable(feature string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(feature))
	return strings.Contains(trimmed, "表格") || strings.Contains(trimmed, "table")
}

func buildFormalReportPrompt(source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, prepared analysisPrepareResult, features []reportFeaturePrompt, validationErrors []string, attempt, maxRetry int) string {
	var b strings.Builder
	b.WriteString("现在请基于你刚才的分析计划与实际代码阅读结果，输出最终分析报告。\n\n")
	if len(validationErrors) > 0 {
		b.WriteString("你上一次输出未通过系统格式校验。请完整重写整份正式报告，不要输出解释、diff、补丁或局部修订。\n")
		if maxRetry > 0 {
			b.WriteString("当前是修订轮次：")
			b.WriteString(strconv.Itoa(attempt))
			b.WriteString("/")
			b.WriteString(strconv.Itoa(maxRetry))
			b.WriteString("。\n")
		}
		b.WriteString("本轮必须修复以下阻塞问题：\n")
		for _, item := range validationErrors {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(trimmed)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("硬性要求：\n")
	b.WriteString("1. 第一行必须直接是 `# GitHub 功能实现原理报告`，标题前禁止输出任何内容。\n")
	b.WriteString("2. 只输出最终 Markdown 报告正文，不要输出额外说明，不要加代码围栏。\n")
	b.WriteString("3. 标题文本、标题层级、标题顺序必须与下面模板完全一致。\n")
	b.WriteString("4. 第三部分的“特点 N”数量由你根据仓库自动决定（至少 1 个），不要凑数。\n")
	b.WriteString("5. 第四部分的“问题 N”必须与关注点一一对应、数量一致、顺序一致。\n")
	b.WriteString("6. 第一到第四部分禁止出现任何 `file:line` / `file:line-line` 引用，也不要贴代码片段。\n")
	b.WriteString("7. 第五部分必须为每个“特点 N”和“问题 N”给出代码定位与证据引用，证据格式必须是 `path/to/file.ext:123`（可选 `:123-130`）。\n")
	b.WriteString("8. 证据必须来自真实仓库文件；不确定时说明不确定，并降低置信度，但仍需给出最相关的定位线索。\n")
	b.WriteString("9. 这条回复会被系统视为正式报告候选；只有通过系统校验后才会成为 Repo Library 正式结果。\n\n")

	b.WriteString("请严格按下面模板输出：\n\n")
	b.WriteString("# GitHub 功能实现原理报告\n\n")
	b.WriteString("## Run 1\n\n")

	b.WriteString("## 第一部分：技术栈与模块语言\n")
	b.WriteString("- 仓库: ")
	b.WriteString(source.RepoURL)
	b.WriteString("\n- 请求 Ref: ")
	b.WriteString(snapshot.RequestedRef)
	b.WriteString("\n- 解析 Ref: ")
	b.WriteString(prepared.ResolvedRef)
	b.WriteString("\n- Commit: ")
	b.WriteString(prepared.CommitSHA)
	b.WriteString("\n- 生成时间: <由你填写当前分析时间>\n")
	b.WriteString("- 主要语言/技术栈总览: <一句话概括>\n")
	b.WriteString("- 后端: <语言 + 框架/库 + 运行方式 + 模块拆分>\n")
	b.WriteString("- 前端: <语言 + UI 框架/组件库 + 状态管理 + 构建工具>\n")
	b.WriteString("- 其它模块: <例如 scripts/services/cli 等，如果没有写无>\n\n")

	b.WriteString("## 第二部分：项目用途与核心特点\n")
	b.WriteString("- 项目做什么用: <中文说明>\n")
	b.WriteString("- 典型使用场景: <中文说明>\n")
	b.WriteString("- 核心特点概览: <用 3-7 条要点概括，不要堆砌名词>\n\n")
	b.WriteString("### 风险与局限\n")
	b.WriteString("- <精简列出 3-7 条高价值风险/局限，不要凑数；没有就写无>\n\n")

	b.WriteString("## 第三部分：特点实现思路\n\n")
	b.WriteString("### 特点 1: <标题>\n")
	b.WriteString("- 动机: <为什么要做>\n")
	b.WriteString("- 目标: <要解决什么问题>\n")
	b.WriteString("- 思路: <用中文讲清实现思路与真实取舍>\n")
	b.WriteString("- 取舍: <代价/限制/边界条件>\n")
	b.WriteString("- 置信度: high|medium|low\n\n")
	b.WriteString("（你可以继续输出 特点 2/3/4...，数量由你根据仓库自动决定，但不要凑数。）\n\n")

	b.WriteString("## 第四部分：提问与解答\n\n")
	for _, feature := range features {
		b.WriteString("### 问题 ")
		b.WriteString(strconv.Itoa(feature.Index))
		b.WriteString(": ")
		b.WriteString(feature.Title)
		b.WriteString("\n")
		b.WriteString("- 结论: <一句话先讲清结论>\n")
		b.WriteString("- 思路: <用中文讲清你理解到的实现逻辑与真实动机>\n")
		b.WriteString("- 取舍: <代价/限制/边界条件>\n")
		b.WriteString("- 置信度: high|medium|low\n\n")
	}

	b.WriteString("## 第五部分：实现定位与证据\n\n")
	b.WriteString("（这一部分允许且必须给出 `file:line` 证据，用于知识库定位。）\n\n")
	b.WriteString("### 特点 1: <标题（必须与第三部分完全一致）>\n")
	b.WriteString("- <file:line> [dimension] - <snippet>\n\n")
	b.WriteString("（按第三部分的特点数量，继续输出 特点 2/3/...，每个特点至少 1 条证据。）\n\n")
	for _, feature := range features {
		b.WriteString("### 问题 ")
		b.WriteString(strconv.Itoa(feature.Index))
		b.WriteString(": ")
		b.WriteString(feature.Title)
		b.WriteString("\n")
		b.WriteString("- <file:line> [dimension] - <snippet>\n\n")
	}

	b.WriteString("请现在直接输出最终报告。")
	return strings.TrimSpace(b.String())
}

func sanitizeReportMarkdown(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```markdown")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}
