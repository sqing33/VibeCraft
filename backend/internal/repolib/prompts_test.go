package repolib

import (
	"strings"
	"testing"

	"vibe-tree/backend/internal/store"
)

func TestBuildFinalReportTurnPromptEnumeratesQuestions(t *testing.T) {
	source := store.RepoSource{RepoURL: "https://github.com/example/repo", Owner: "example", Repo: "repo"}
	analysis := store.RepoAnalysisResult{RequestedRef: "main", Depth: "deep", Language: "zh", Features: []string{"1. 多 agent 并行编排如何实现", "2、不同 AI 如何调用（原生 SDK 还是 CLI），以及本地文件修改路径如何确定，用表格展示"}}
	prepared := analysisPrepareResult{ResolvedRef: "main", CommitSHA: "abc123", SourceDir: "/tmp/source", CodeIndexPath: "/tmp/code_index.json"}

	prompt := buildFinalReportTurnPrompt(source, analysis, prepared)

	checks := []string{
		"# GitHub 功能实现原理报告",
		"## 第一部分：技术栈与模块语言",
		"## 第二部分：项目用途与核心特点",
		"## 第三部分：特点实现思路",
		"## 第四部分：提问与解答",
		"### 问题 1: 多 agent 并行编排如何实现",
		"### 问题 2: 不同 AI 如何调用（原生 SDK 还是 CLI），以及本地文件修改路径如何确定，用表格展示",
		"## 第五部分：实现定位与证据",
	}
	for _, item := range checks {
		if !strings.Contains(prompt, item) {
			t.Fatalf("prompt missing %q\n%s", item, prompt)
		}
	}
	if strings.Contains(prompt, "问题 2: 2、") {
		t.Fatalf("expected prompt to normalize question title, got: %s", prompt)
	}
}

func TestBuildReportRepairTurnPromptIncludesValidationErrors(t *testing.T) {
	source := store.RepoSource{RepoURL: "https://github.com/example/repo", Owner: "example", Repo: "repo"}
	analysis := store.RepoAnalysisResult{RequestedRef: "main", Depth: "deep", Language: "zh", Features: []string{"worktree 如何实现"}}
	prepared := analysisPrepareResult{ResolvedRef: "main", CommitSHA: "abc123", SourceDir: "/tmp/source", CodeIndexPath: "/tmp/code_index.json"}

	prompt := buildReportRepairTurnPrompt(source, analysis, prepared, []string{"缺少 `## 第二部分` 标题", "feature 数量不匹配"}, 2, 5)
	if !strings.Contains(prompt, "修订轮次：2/5") {
		t.Fatalf("expected retry counter in repair prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "- 缺少 `## 第二部分` 标题") {
		t.Fatalf("expected validation errors in repair prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "只输出最终 Markdown 报告正文") {
		t.Fatalf("expected final report constraints to remain\n%s", prompt)
	}
}
