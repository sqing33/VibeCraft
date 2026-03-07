package repolib

import (
	"fmt"
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

func buildFinalReportTurnPrompt(source store.RepoSource, snapshot store.RepoSnapshot, run store.RepoAnalysisRun, prepared analysisPrepareResult) string {
	features := strings.Join(run.Features, "；")
	return strings.TrimSpace(fmt.Sprintf(`现在请基于你刚才的分析计划与实际代码阅读结果，输出最终分析报告。

硬性要求：
1. 只输出最终 Markdown 报告正文，不要输出额外说明，不要加代码围栏。
2. 报告必须使用以下结构与标题，保持标题文本一致：

# GitHub 功能实现原理报告

## Run 1

### 第一部分：项目参数与结构解析

#### 元数据
- 仓库: %s
- 请求 Ref: %s
- 解析 Ref: %s
- Commit: %s
- 生成时间: <由你填写当前分析时间>
- 分析深度: %s
- 分析模式: ai-chat
- 源码目录: %s
- 索引文件: %s
- 子代理结果: 无

#### 仓库结构心智模型
- 文件总数: <如果无法精确得到可保守说明>
- 可检索文本文件: <如果无法精确得到可保守说明>
- 主要语言: <按你观察填写>
- 运行入口线索: <入口文件列表>
- 模块边界线索: <目录/模块边界列表>

#### 项目特点与标志实现
- README-first: <README 路径或无>

##### 项目特点 1: <标题>
- 来源: readme 或 inference
- README 线索: <线索或无>
- 实现机制: <中文解释>
- 置信度: high|medium|low
- 关键证据引用:
  - <file:line>

##### 项目特点 2: <标题>
...

### 第二部分：面向人的功能说明

#### 功能 1: %s
- 功能作用: <中文说明>
- 特殊功能: <中文说明>
- 实现想法: <中文说明>
- 置信度: high|medium|low
- 关键证据引用:
  - <file:line>

### 第三部分：面向 AI 的实现细节与证据链

#### 面向 AI 的实现细节

##### 功能 1: %s

###### 运行时控制流
- <结论>
- 置信度: high|medium|low
- inference: true|false

###### 数据流
- <结论>
- 置信度: high|medium|low
- inference: true|false

###### 状态与生命周期
- <结论>
- 置信度: high|medium|low
- inference: true|false

###### 失败与恢复
- <结论>
- 置信度: high|medium|low
- inference: true|false

###### 并发与时序
- <结论>
- 置信度: high|medium|low
- inference: true|false

###### 关键证据
- <file:line> [dimension] - <snippet>

###### 推断与未知点
- <未知点>

#### 跨功能耦合与系统风险
- <风险条目>

3. 你必须真实引用仓库中的 file:line 证据；找不到时再标记 inference。
4. 不要把 README 标题机械地当成项目特点；项目特点必须是你归纳后的工程特征。
5. 如果“技术栈”之类问题没有证据，就不要硬写成一个功能。
6. 关注点是：%s
7. 这条回复会被系统视为“正式报告版本”，后续普通追问不自动替代它；只有显式同步动作才会覆盖正式结果。

请现在直接输出最终报告。`, source.RepoURL, snapshot.RequestedRef, prepared.ResolvedRef, prepared.CommitSHA, run.Depth, prepared.SourceDir, prepared.CodeIndexPath, features, features, features))
}

func sanitizeReportMarkdown(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```markdown")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}
