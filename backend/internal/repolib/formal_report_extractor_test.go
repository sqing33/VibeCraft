package repolib

import "testing"

func TestExtractFormalReportCardsV2QuestionConclusionAndSummary(t *testing.T) {
	report := `# GitHub 功能实现原理报告

## Run 1

## 第一部分：技术栈与模块语言
- 仓库: openakita
- 请求 Ref: main
- 解析 Ref: main
- Commit: abc123
- 生成时间: 2026-03-11T12:00:00Z
- 主要语言/技术栈总览: Go, React
- 后端: Go
- 前端: React
- 其它模块: -

## 第二部分：项目用途与核心特点
- 项目做什么用: x
- 典型使用场景: x
- 核心特点概览: x

### 风险与局限
- x

## 第三部分：特点实现思路
### 特点 1: 主流程
- 动机: x
- 目标: x
- 思路: x
- 取舍: x
- 置信度: high

## 第四部分：提问与解答
### 问题 1: 多 Agent 并行机制：任务拆分、调度、路由/接力、结果合并/冲突处理
- 结论: 任务拆分、调度、路由/接力和结果合并由统一编排层负责。
- 思路: 主 Agent 先拆任务并分配给多个 worker，再按结果回收与合并。第二句不会进入 summary。
- 取舍: 牺牲少量延迟换取更稳定的上下文隔离。
- 置信度: high

## 第五部分：实现定位与证据
### 问题 1: 多 Agent 并行机制：任务拆分、调度、路由/接力、结果合并/冲突处理
- internal/orchestrator/router.go:42 [control-flow] - 统一调度入口
`

	cards, totalCards, totalEvidence := extractFormalReportCardsV2(report, []string{"多 Agent 并行机制"})
	if totalCards != 2 {
		t.Fatalf("expected 2 cards, got %d", totalCards)
	}
	if totalEvidence != 1 {
		t.Fatalf("expected 1 evidence, got %d", totalEvidence)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 card inputs, got %d", len(cards))
	}

	questionCard := cards[1]
	if questionCard.Title != "多 Agent 并行机制" {
		t.Fatalf("expected normalized feature title, got %q", questionCard.Title)
	}
	if questionCard.Conclusion == nil || *questionCard.Conclusion != "任务拆分、调度、路由/接力和结果合并由统一编排层负责。" {
		t.Fatalf("unexpected conclusion: %#v", questionCard.Conclusion)
	}
	if questionCard.Summary != "" {
		t.Fatalf("expected redundant summary to be suppressed, got %q", questionCard.Summary)
	}
	if questionCard.Mechanism == nil || *questionCard.Mechanism == "" {
		t.Fatalf("expected mechanism")
	}
	if len(questionCard.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(questionCard.Evidence))
	}
}
