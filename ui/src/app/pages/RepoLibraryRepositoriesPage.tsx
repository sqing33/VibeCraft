import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Selection } from '@react-types/shared'
import { Alert, Button, Chip, Input, Select, SelectItem, Skeleton, Textarea } from '@heroui/react'
import { FolderGit2, RefreshCcw, Search, Sparkles } from 'lucide-react'

import {
  goToRepoLibraryPatternSearch,
  goToRepoLibraryRepository,
} from '@/app/routes'
import {
  createRepoLibraryAnalysis,
  fetchCLIToolSettings,
  fetchRepoLibraryRepositories,
  type CLITool,
  type LLMModelProfile,
  type RepoLibraryAnalysisRequest,
  type RepoLibraryAnalysisRun,
  type RepoLibraryDepth,
  type RepoLibraryCreateAnalysisResponse,
  type RepoLibraryRepositorySummary,
} from '@/lib/daemon'
import { buildCLIToolModelProfiles, cliToolDefaultModelID } from '@/lib/cliToolModels'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import { RepoLibraryLayout } from './RepoLibraryLayout'

function selectionToString(keys: Selection) {
  if (keys === 'all') return ''
  const first = keys.values().next().value
  return typeof first === 'string' ? first : ''
}

function parseFeatureList(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function formatAnalysisStatus(status: string): string {
  if (status === 'queued') return '排队中'
  if (status === 'running') return '分析中'
  if (status === 'succeeded') return '已完成'
  if (status === 'failed') return '失败'
  return status || '未知'
}

function analysisStatusColor(status: string): 'default' | 'success' | 'danger' | 'warning' {
  if (status === 'succeeded') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'queued' || status === 'running') return 'warning'
  return 'default'
}

function EmptyRepositories() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-28 w-full rounded-xl" />
      <Skeleton className="h-28 w-full rounded-xl" />
      <Skeleton className="h-28 w-full rounded-xl" />
    </div>
  )
}

function RepositoryCard(props: { item: RepoLibraryRepositorySummary }) {
  const { item } = props
  const latestAnalysis = item.latest_analysis ?? null
  const latestSnapshot = item.latest_snapshot ?? null

  return (
    <button
      type="button"
      className="w-full rounded-2xl border bg-card p-4 text-left shadow-sm transition hover:border-primary/40 hover:bg-muted/20"
      onClick={() => goToRepoLibraryRepository(item.repository_id)}
    >
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-base font-semibold">{item.full_name || item.repo_url}</div>
            {latestAnalysis ? (
              <Chip color={analysisStatusColor(latestAnalysis.status)} variant="flat" size="sm">
                {formatAnalysisStatus(latestAnalysis.status)}
              </Chip>
            ) : null}
          </div>
          <div className="text-sm text-muted-foreground">{item.repo_url}</div>
          {item.description ? <div className="text-sm text-muted-foreground">{item.description}</div> : null}
        </div>

        <div className="flex flex-wrap gap-2">
          {typeof item.snapshot_count === 'number' ? (
            <Chip variant="bordered" size="sm">
              快照 {item.snapshot_count}
            </Chip>
          ) : null}
          {typeof item.card_count === 'number' ? (
            <Chip variant="bordered" size="sm">
              卡片 {item.card_count}
            </Chip>
          ) : null}
          {item.default_branch ? (
            <Chip variant="bordered" size="sm">
              默认分支 {item.default_branch}
            </Chip>
          ) : null}
        </div>
      </div>

      <div className="mt-4 grid gap-3 text-sm text-muted-foreground md:grid-cols-3">
        <div className="rounded-xl border bg-muted/20 p-3">
          <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最新 Ref</div>
          <div className="mt-1 font-medium text-foreground">
            {latestSnapshot?.resolved_ref || latestSnapshot?.ref || item.latest_ref || '暂无'}
          </div>
        </div>
        <div className="rounded-xl border bg-muted/20 p-3">
          <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最近提交</div>
          <div className="mt-1 font-medium text-foreground">
            {latestSnapshot?.commit_sha?.slice(0, 12) || item.latest_commit_sha?.slice(0, 12) || '暂无'}
          </div>
        </div>
        <div className="rounded-xl border bg-muted/20 p-3">
          <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最近活动</div>
          <div className="mt-1 font-medium text-foreground">
            {formatRelativeTime(item.updated_at || latestAnalysis?.updated_at || latestSnapshot?.created_at || 0)}
          </div>
        </div>
      </div>
    </button>
  )
}

/**
 * 功能：展示 Repo Library 仓库列表，并允许用户提交新的仓库分析任务。
 * 参数/返回：无入参；返回仓库列表页与 Analyze Repo 表单。
 * 失败场景：仓库列表或分析创建失败时展示错误提示，并允许用户重试。
 * 副作用：发起仓库列表查询与分析创建请求；成功后刷新列表并更新 hash 路由入口。
 */
export function RepoLibraryRepositoriesPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)

  const [items, setItems] = useState<RepoLibraryRepositorySummary[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [repoUrl, setRepoUrl] = useState('')
  const [ref, setRef] = useState('HEAD')
  const [featuresText, setFeaturesText] = useState('认证流程\n仓库路由与导航')
  const [depth, setDepth] = useState<RepoLibraryDepth>('standard')
  const [language, setLanguage] = useState<'zh-CN' | 'en'>('zh-CN')
  const [analyzerMode, setAnalyzerMode] = useState<'full' | 'compact'>('full')
  const [submitting, setSubmitting] = useState(false)
  const [created, setCreated] = useState<RepoLibraryCreateAnalysisResponse | null>(null)
  const [cliTools, setCliTools] = useState<CLITool[]>([])
  const [toolModels, setToolModels] = useState<LLMModelProfile[]>([])
  const [selectedCliToolId, setSelectedCliToolId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setItems(await fetchRepoLibraryRepositories(daemonUrl))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    let cancelled = false
    void fetchCLIToolSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return
        setCliTools(settings.tools ?? [])
        setToolModels(settings.models ?? [])
      })
      .catch(() => {
        if (cancelled) return
        setCliTools([])
        setToolModels([])
      })
    return () => {
      cancelled = true
    }
  }, [daemonUrl])

  const selectableTools = useMemo(() => cliTools.filter((tool) => tool.enabled), [cliTools])
  const toolsById = useMemo(() => {
    const next = new Map<string, CLITool>()
    for (const tool of selectableTools) next.set(tool.id, tool)
    return next
  }, [selectableTools])
  const effectiveCliToolId =
    selectedCliToolId && selectableTools.some((tool) => tool.id === selectedCliToolId)
      ? selectedCliToolId
      : selectableTools[0]?.id ?? ''
  const modelsForTool = useCallback(
    (toolId: string) => {
      const tool = toolsById.get(toolId)
      return buildCLIToolModelProfiles(tool, toolModels)
    },
    [toolModels, toolsById],
  )
  const effectiveModelId = useMemo(() => {
    const models = modelsForTool(effectiveCliToolId)
    if (selectedModelId && models.some((model) => model.id === selectedModelId)) return selectedModelId
    const tool = toolsById.get(effectiveCliToolId)
    const fallback = cliToolDefaultModelID(tool, toolModels)
    if (fallback && models.some((model) => model.id === fallback)) return fallback
    return models[0]?.id ?? ''
  }, [effectiveCliToolId, modelsForTool, selectedModelId, toolModels, toolsById])

  const onSubmit = async () => {
    const features = parseFeatureList(featuresText)
    if (!repoUrl.trim()) {
      toast({ variant: 'destructive', title: '请输入 GitHub 仓库地址' })
      return
    }
    if (features.length === 0) {
      toast({ variant: 'destructive', title: '请至少填写一个分析特征' })
      return
    }

    const req: RepoLibraryAnalysisRequest = {
      repo_url: repoUrl.trim(),
      ref: ref.trim() || 'HEAD',
      features,
      depth,
      language,
      analyzer_mode: analyzerMode,
      cli_tool_id: effectiveCliToolId || undefined,
      model_id: effectiveModelId || undefined,
    }

    setSubmitting(true)
    try {
      const next = await createRepoLibraryAnalysis(daemonUrl, req)
      setCreated(next)
      toast({
        title: '分析任务已创建',
        description: next.analysis.execution_id || next.analysis.analysis_id,
      })
      await refresh()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '创建分析失败', description: message })
    } finally {
      setSubmitting(false)
    }
  }

  const createdAnalysis: RepoLibraryAnalysisRun | null = created?.analysis ?? null
  const createdRepositoryId = created?.repository?.repository_id ?? null

  return (
    <RepoLibraryLayout
      activeNav="repositories"
      title="Repo Library 仓库"
      description="集中管理外部仓库分析结果、快照与知识卡片。你可以先提交一个 GitHub 仓库分析，再进入详情查看快照、报告、卡片和执行日志。"
      meta={
        <Chip variant="flat" color={health.status === 'ok' ? 'success' : 'default'}>
          {health.status === 'ok' ? 'Daemon 已连接' : 'Daemon 未就绪'}
        </Chip>
      }
      actions={
        <>
          <Button variant="flat" size="sm" startContent={<Search className="h-4 w-4" />} onPress={goToRepoLibraryPatternSearch}>
            去模式搜索
          </Button>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh()}>
            刷新列表
          </Button>
        </>
      }
    >
      <section className="rounded-2xl border bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Sparkles className="h-5 w-5" />
              Analyze Repo
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              提交 GitHub 仓库 URL、Ref、分析深度和关注特征，后端会异步执行 analyzer 并沉淀为 Repo Library 资产。
            </div>
          </div>
          <Chip variant="bordered" size="sm">
            支持异步分析
          </Chip>
        </div>

        <div className="grid gap-3 md:grid-cols-[1.4fr_0.8fr_0.6fr]">
          <Input
            label="仓库地址"
            placeholder="https://github.com/owner/repo"
            value={repoUrl}
            onValueChange={setRepoUrl}
          />
          <Input label="Ref" placeholder="HEAD / main / v1.0.0" value={ref} onValueChange={setRef} />
          <Select
            aria-label="分析深度"
            label="分析深度"
            description={depth === 'deep' ? '深度分析：适合更深入的实现机制与证据链梳理。' : '标准分析：适合常规结构理解与功能概览。'}
            selectedKeys={new Set([depth])}
            selectionMode="single"
            disallowEmptySelection
            onSelectionChange={(keys) => {
              const value = selectionToString(keys)
              if (value === 'deep' || value === 'standard') setDepth(value)
            }}
          >
            <SelectItem key="standard">标准</SelectItem>
            <SelectItem key="deep">深度</SelectItem>
          </Select>
        </div>

        <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-xl border bg-muted/20 p-3">
            <div className="mb-2 text-sm font-medium">输出语言</div>
            <div className="flex flex-wrap gap-2">
              <Button variant={language === 'zh-CN' ? 'flat' : 'light'} size="sm" onPress={() => setLanguage('zh-CN')}>
                中文
              </Button>
              <Button variant={language === 'en' ? 'flat' : 'light'} size="sm" onPress={() => setLanguage('en')}>
                English
              </Button>
            </div>
          </div>
          <div className="rounded-xl border bg-muted/20 p-3">
            <div className="mb-2 text-sm font-medium">分析模式</div>
            <div className="flex flex-wrap gap-2">
              <Button variant={analyzerMode === 'full' ? 'flat' : 'light'} size="sm" onPress={() => setAnalyzerMode('full')}>
                完整分析
              </Button>
              <Button variant={analyzerMode === 'compact' ? 'flat' : 'light'} size="sm" onPress={() => setAnalyzerMode('compact')}>
                快速概览
              </Button>
            </div>
          </div>
          <Select
            aria-label="CLI 工具"
            label="CLI 工具"
            placeholder={selectableTools.length === 0 ? '暂无可用 CLI 工具' : '选择分析工具'}
            description={effectiveCliToolId ? `当前协议：${toolsById.get(effectiveCliToolId)?.protocol_family || '未知'}` : '沿用 Settings → CLI 工具 中启用的工具'}
            selectedKeys={effectiveCliToolId ? new Set([effectiveCliToolId]) : new Set()}
            selectionMode="single"
            disallowEmptySelection
            isDisabled={submitting || selectableTools.length === 0}
            onSelectionChange={(keys) => {
              const value = selectionToString(keys)
              if (!value) return
              setSelectedCliToolId(value)
            }}
          >
            {selectableTools.map((tool) => (
              <SelectItem key={tool.id}>{tool.label}</SelectItem>
            ))}
          </Select>
          <Select
            aria-label="模型"
            label="模型"
            placeholder={modelsForTool(effectiveCliToolId).length === 0 ? '当前工具暂无可用模型' : '选择模型'}
            description={effectiveModelId ? `默认回退：${effectiveModelId}` : '模型列表按 CLI 工具协议自动过滤'}
            selectedKeys={effectiveModelId ? new Set([effectiveModelId]) : new Set()}
            selectionMode="single"
            disallowEmptySelection
            isDisabled={submitting || modelsForTool(effectiveCliToolId).length === 0}
            onSelectionChange={(keys) => {
              const value = selectionToString(keys)
              if (!value) return
              setSelectedModelId(value)
            }}
          >
            {modelsForTool(effectiveCliToolId).map((model) => (
              <SelectItem key={model.id}>{model.label || model.id} · {model.model}</SelectItem>
            ))}
          </Select>
        </div>

        <div className="mt-3">
          <Textarea
            label="分析特征 / 问题"
            placeholder="每行一个，例如：\n认证流程\n仓库路由与导航\n数据层抽象"
            minRows={4}
            value={featuresText}
            onValueChange={setFeaturesText}
          />
        </div>

        <div className="mt-4 flex justify-end">
          <Button color="primary" isLoading={submitting} onPress={onSubmit}>
            创建分析任务
          </Button>
        </div>
      </section>

      {createdAnalysis ? (
        <section className="rounded-2xl border bg-card p-5 shadow-sm">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <div className="text-sm font-semibold">最近提交的分析</div>
                <Chip color={analysisStatusColor(createdAnalysis.status)} variant="flat" size="sm">
                  {formatAnalysisStatus(createdAnalysis.status)}
                </Chip>
              </div>
              <div className="text-sm text-muted-foreground">{createdAnalysis.repo_url}</div>
              <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-3">
                <div>
                  <div className="text-muted-foreground/80">分析 ID</div>
                  <code className="text-foreground">{createdAnalysis.analysis_id}</code>
                </div>
                <div>
                  <div className="text-muted-foreground/80">Execution</div>
                  <code className="text-foreground">{createdAnalysis.execution_id || '待分配'}</code>
                </div>
                <div>
                  <div className="text-muted-foreground/80">最近更新时间</div>
                  <div className="text-foreground">{formatRelativeTime(createdAnalysis.updated_at || 0)}</div>
                </div>
              </div>
            </div>

            {createdRepositoryId ? (
              <Button
                variant="flat"
                size="sm"
                onPress={() => goToRepoLibraryRepository(createdRepositoryId)}
              >
                打开仓库详情
              </Button>
            ) : null}
          </div>
        </section>
      ) : null}

      <section className="space-y-3">
        <div className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
          <FolderGit2 className="h-4 w-4" />
          已收录仓库
        </div>

        {error ? (
          <Alert color="danger" title="加载仓库列表失败" description={error} />
        ) : loading && items.length === 0 ? (
          <EmptyRepositories />
        ) : items.length === 0 ? (
          <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">
            还没有仓库分析结果。先从上面的表单提交一个 GitHub 仓库试试。
          </div>
        ) : (
          <div className="space-y-3">
            {items.map((item) => (
              <RepositoryCard key={item.repository_id} item={item} />
            ))}
          </div>
        )}
      </section>
    </RepoLibraryLayout>
  )
}
