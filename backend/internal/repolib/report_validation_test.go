package repolib

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vibe-tree/backend/internal/store"
)

const validationTestFeatureRaw1 = "1. 为什么需要正式报告强校验"
const validationTestFeature1 = "为什么需要正式报告强校验"
const validationTestFeatureRaw2 = "2、校验失败后会自动重试吗"
const validationTestFeature2 = "校验失败后会自动重试吗"

const validCanonicalReportV2 = `# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: https://github.com/example/repo
- 请求 Ref: main
- 解析 Ref: main
- Commit: abc123
- 生成时间: 2026-03-10
- 主要语言/技术栈总览: Go + React + Python
- 后端: Go daemon (Gin/SQLite)
- 前端: React + TypeScript
- 其它模块: Python analyzer (extract/validate/search)

## 第二部分：项目用途与核心特点
- 项目做什么用: 对真实仓库做自动分析，产出可检索的结构化报告与知识卡片。
- 典型使用场景: 选择仓库 + 关注点，生成“实现思路 + 定位证据”。
- 核心特点概览: 报告强校验、失败自动重试、抽卡/搜索统一入口。

### 风险与局限
- 校验过严可能增加重试次数。
- 输出格式变更需要同步更新抽卡器。

## 第三部分：特点实现思路

### 特点 1: 报告强校验
- 动机: 正式报告需要被程序解析，否则卡片会是空的。
- 目标: 强制输出稳定结构，保证抽卡和搜索可用。
- 思路: 先固定模板，再用脚本检查标题/字段/证据，并把错误回灌给 AI 重写。
- 取舍: 模板更严格，会增加生成成本。
- 置信度: high

## 第四部分：提问与解答

### 问题 1: ` + validationTestFeature1 + `
- 结论: 通过固定模板 + validate-report 脚本保证。
- 思路: 先限制结构，再用机器校验拦截无效输出。
- 取舍: 规则越严越容易触发重试，但最终更稳定。
- 置信度: high

### 问题 2: ` + validationTestFeature2 + `
- 结论: 会；失败会带着错误信息让 AI 完整重写。
- 思路: 把 validator 的阻塞错误作为 prompt 的输入，让 AI 针对性修复。
- 取舍: 重试会增加分析耗时。
- 置信度: medium

## 第五部分：实现定位与证据

### 特点 1: 报告强校验
- services/repo-analyzer/app/validate_report.py:120 [validator] - required headings

### 问题 1: ` + validationTestFeature1 + `
- services/repo-analyzer/app/cli.py:60 [cli] - validate-report command

### 问题 2: ` + validationTestFeature2 + `
- backend/internal/repolib/report_validation.go:40 [retry] - retry loop
`

const invalidWrongLevelsReportV2 = `# GitHub 功能实现原理报告

## Run 1

### 第一部分：技术栈与模块语言
- 仓库: https://github.com/example/repo

## 第二部分：项目用途与核心特点
- 项目做什么用: something

### 风险与局限
- 无

## 第三部分：特点实现思路

### 特点 1: x
- 动机: x
- 目标: x
- 思路: x
- 取舍: x
- 置信度: high

## 第四部分：提问与解答

### 问题 1: ` + validationTestFeature1 + `
- 结论: x
- 思路: x
- 取舍: x
- 置信度: high

### 问题 2: ` + validationTestFeature2 + `
- 结论: x
- 思路: x
- 取舍: x
- 置信度: high

## 第五部分：实现定位与证据

### 特点 1: x
- services/repo-analyzer/app/validate_report.py:120 [validator] - required headings

### 问题 1: ` + validationTestFeature1 + `
- services/repo-analyzer/app/cli.py:60 [cli] - validate-report command

### 问题 2: ` + validationTestFeature2 + `
- backend/internal/repolib/report_validation.go:40 [retry] - retry loop
`

func TestValidateCandidateReportAcceptsCanonicalReportV2(t *testing.T) {
	service := newValidationTestService(t)
	layout := newValidationTestLayout(t)
	source := store.RepoSource{RepoURL: "https://github.com/example/repo", RepoKey: "example-repo"}
	snapshot := store.RepoSnapshot{ID: "rp_demo"}
	run := store.RepoAnalysisRun{ID: "rr_demo", Features: []string{validationTestFeatureRaw1, validationTestFeatureRaw2}}
	outputPath := filepath.Join(layout.DerivedDir, "report.validation.json")

	result, err := service.validateCandidateReport(context.Background(), source, snapshot, run, layout, validCanonicalReportV2, filepath.Join(layout.DerivedDir, "report.md"), outputPath)
	if err != nil {
		t.Fatalf("validateCandidateReport returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid report, got errors: %v", result.Errors)
	}
	if result.CardCount <= 0 {
		t.Fatalf("expected card_count > 0, got %d", result.CardCount)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected validation output file: %v", err)
	}
}

func TestValidateCandidateReportRejectsWrongHeadingLevelsV2(t *testing.T) {
	service := newValidationTestService(t)
	layout := newValidationTestLayout(t)
	source := store.RepoSource{RepoURL: "https://github.com/example/repo", RepoKey: "example-repo"}
	snapshot := store.RepoSnapshot{ID: "rp_demo"}
	run := store.RepoAnalysisRun{ID: "rr_demo", Features: []string{validationTestFeatureRaw1, validationTestFeatureRaw2}}

	result, err := service.validateCandidateReport(context.Background(), source, snapshot, run, layout, invalidWrongLevelsReportV2, filepath.Join(layout.DerivedDir, "report.invalid.md"), filepath.Join(layout.DerivedDir, "report.validation.json"))
	if err != nil {
		t.Fatalf("validateCandidateReport returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid report, got valid result")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected validation errors for wrong heading levels")
	}
}

func newValidationTestService(t *testing.T) *Service {
	t.Helper()
	projectRoot, err := discoverProjectRoot()
	if err != nil {
		t.Fatalf("discoverProjectRoot failed: %v", err)
	}
	return &Service{projectRoot: projectRoot, pythonBin: "python3"}
}

func newValidationTestLayout(t *testing.T) pipelineLayout {
	t.Helper()
	root := t.TempDir()
	derived := filepath.Join(root, "derived")
	if err := os.MkdirAll(derived, 0o755); err != nil {
		t.Fatalf("mkdir derived: %v", err)
	}
	return pipelineLayout{
		SnapshotDir:  root,
		DerivedDir:   derived,
		ReportPath:   filepath.Join(root, "report.md"),
		ArtifactsDir: filepath.Join(root, "artifacts"),
	}
}
