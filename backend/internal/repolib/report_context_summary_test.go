package repolib

import "testing"

func TestExtractReportContextSummary(t *testing.T) {
	report := `# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: openakita/openakita
- 请求 Ref: HEAD
- 解析 Ref: HEAD
- Commit: abc123
- 生成时间: 2026-03-11 16:01:56 +0800
- 主要语言/技术栈总览: Python + FastAPI + React + TypeScript
- 后端: Python 3.11 + FastAPI + asyncio
- 前端: React 18 + TypeScript + Vite
- 其它模块: Tauri / Capacitor / MCP

## 第二部分：项目用途与核心特点
- 项目做什么用: x
- 典型使用场景: x
- 核心特点概览: x
`

	summary, ok := extractReportContextSummary(report)
	if !ok || summary == nil {
		t.Fatalf("expected summary extraction to succeed")
	}
	if summary.GeneratedAt == nil || *summary.GeneratedAt != "2026-03-11 16:01:56 +0800" {
		t.Fatalf("unexpected generated_at: %#v", summary.GeneratedAt)
	}
	if summary.StackOverview == nil || *summary.StackOverview != "Python + FastAPI + React + TypeScript" {
		t.Fatalf("unexpected stack_overview: %#v", summary.StackOverview)
	}
	if summary.BackendSummary == nil || *summary.BackendSummary != "Python 3.11 + FastAPI + asyncio" {
		t.Fatalf("unexpected backend_summary: %#v", summary.BackendSummary)
	}
	if summary.FrontendSummary == nil || *summary.FrontendSummary != "React 18 + TypeScript + Vite" {
		t.Fatalf("unexpected frontend_summary: %#v", summary.FrontendSummary)
	}
	if summary.OtherModulesSummary == nil || *summary.OtherModulesSummary != "Tauri / Capacitor / MCP" {
		t.Fatalf("unexpected other_modules_summary: %#v", summary.OtherModulesSummary)
	}
}

func TestExtractReportContextSummaryMissingSection(t *testing.T) {
	if summary, ok := extractReportContextSummary("# GitHub 功能实现原理报告\n\n## Run 1\n"); ok || summary != nil {
		t.Fatalf("expected summary extraction to fail")
	}
}
