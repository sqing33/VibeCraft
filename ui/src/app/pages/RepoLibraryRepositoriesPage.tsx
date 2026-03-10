import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Selection } from '@react-types/shared'
import { Alert, Button, Chip, Input, Select, SelectItem, Skeleton, Textarea } from '@heroui/react'
import { Plus, RefreshCcw, Search, Sparkles } from 'lucide-react'

import { goToRepoLibraryPatternSearch, goToRepoLibraryRepository } from '@/app/routes'
import {
  cliToolProtocolFamilies,
  createRepoLibraryAnalysis,
  fetchCLIToolSettings,
  fetchRepoLibraryRepositories,
  fetchRuntimeModelSettings,
  type CLITool,
  type RepoLibraryAnalysisRequest,
  type RepoLibraryAnalysisRun,
  type RepoLibraryCreateAnalysisResponse,
  type RepoLibraryDepth,
  type RuntimeModelSettings,
} from '@/lib/daemon'
import {
  runtimeDefaultModelForToolId,
  runtimeModelsForToolId,
} from '@/lib/runtimeModels'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'
import { useRepoLibraryUIStore } from '@/stores/repoLibraryUIStore'

import { LoadingVeil } from '@/app/components/LoadingVeil'
import { RepoLibraryShell, RepoLibrarySidebarRepositoryItem } from '@/app/components/RepoLibraryShell'

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
    <div className="space-y-2">
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
    </div>
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

  const items = useRepoLibraryUIStore((s) => s.repositories)
  const repositoriesLoaded = useRepoLibraryUIStore((s) => s.repositoriesLoaded)
  const loading = useRepoLibraryUIStore((s) => s.repositoriesRefreshing)
  const error = useRepoLibraryUIStore((s) => s.repositoriesError)
  const setRepositoriesState = useRepoLibraryUIStore((s) => s.setRepositoriesState)

  const [repoUrl, setRepoUrl] = useState('')
  const [ref, setRef] = useState('HEAD')
  const [featuresText, setFeaturesText] = useState('认证流程\n仓库路由与导航')
  const [depth, setDepth] = useState<RepoLibraryDepth>('standard')
  const [language, setLanguage] = useState<'zh-CN' | 'en'>('zh-CN')
  const [analyzerMode, setAnalyzerMode] = useState<'full' | 'compact'>('full')
  const [submitting, setSubmitting] = useState(false)
  const [created, setCreated] = useState<RepoLibraryCreateAnalysisResponse | null>(null)
  const [cliTools, setCliTools] = useState<CLITool[]>([])
  const [runtimeModelSettings, setRuntimeModelSettings] = useState<RuntimeModelSettings | null>(null)
  const [selectedCliToolId, setSelectedCliToolId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')

  const hasRepositoryCache = useMemo(
    () => repositoriesLoaded || items.length > 0,
    [items.length, repositoriesLoaded],
  )

  const refresh = useCallback(async (options?: { force?: boolean }) => {
    const force = options?.force ?? false
    if (!force && useRepoLibraryUIStore.getState().repositoriesRefreshing) return

    setRepositoriesState({ refreshing: true, error: null })
    try {
      setRepositoriesState({
        repositories: await fetchRepoLibraryRepositories(daemonUrl),
        loaded: true,
        refreshing: false,
        error: null,
      })
    } catch (err: unknown) {
      setRepositoriesState({ refreshing: false, error: err instanceof Error ? err.message : String(err) })
    }
  }, [daemonUrl, setRepositoriesState])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      fetchCLIToolSettings(daemonUrl),
      fetchRuntimeModelSettings(daemonUrl),
    ])
      .then(([cliSettings, runtimeSettings]) => {
        if (cancelled) return
        setCliTools(cliSettings.tools ?? [])
        setRuntimeModelSettings(runtimeSettings)
      })
      .catch(() => {
        if (cancelled) return
        setCliTools([])
        setRuntimeModelSettings(null)
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
    (toolId: string) => runtimeModelsForToolId(runtimeModelSettings, toolId),
    [runtimeModelSettings],
  )
  const effectiveModelId = useMemo(() => {
    const models = modelsForTool(effectiveCliToolId)
    if (selectedModelId && models.some((model) => model.id === selectedModelId)) return selectedModelId
    const fallback = runtimeDefaultModelForToolId(runtimeModelSettings, effectiveCliToolId)
    if (fallback && models.some((model) => model.id === fallback)) return fallback
    return models[0]?.id ?? ''
  }, [effectiveCliToolId, modelsForTool, runtimeModelSettings, selectedModelId])

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

  const sidebarContent = (
    <div className="relative min-h-[120px]">
      {error && !hasRepositoryCache ? <Alert color="danger" title="加载失败" description={error} className="mt-0" /> : null}
      {!hasRepositoryCache && loading ? (
        <EmptyRepositories />
      ) : items.length === 0 ? (
        <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
          还没有仓库分析结果，先添加一个 GitHub 仓库试试。
        </div>
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <RepoLibrarySidebarRepositoryItem
              key={item.repository_id}
              title={item.full_name || item.name || item.repo_url}
              subtitle={item.default_branch ? `默认分支：${item.default_branch}` : item.repo_url}
              meta={item.updated_at ? formatRelativeTime(item.updated_at) : '未知'}
              onPress={() => goToRepoLibraryRepository(item.repository_id)}
            />
          ))}
        </div>
      )}
      <LoadingVeil visible={loading && hasRepositoryCache} />
    </div>
  )

  return (
    <RepoLibraryShell
      title="Repo Library"
      headerMeta={<div className="text-sm text-muted-foreground">统一查看分析过的 GitHub 仓库，并在右侧快速提交新的特征分析任务。</div>}
      sidebarTitle="已分析仓库"
      sidebarCount={items.length}
      sidebarAction={
        <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh({ force: true })}>
          刷新
        </Button>
      }
      sidebarContent={sidebarContent}
    >
      <div className="space-y-4">
        <div className="rounded-2xl border border-default-200/70 bg-background/70 p-4 shadow-sm">
          <div className="mb-3 flex items-center justify-between gap-2">
            <div>
              <div className="text-base font-semibold">Analyze Repo</div>
              <div className="text-sm text-muted-foreground">调用后端 GitHub Feature Analyzer 流水线，对目标仓库做多维度特征分析。</div>
            </div>
            <Chip variant="flat" color="primary">
              {selectableTools.length > 0 ? `${selectableTools.length} 个 CLI 可用` : '未配置 CLI'}
            </Chip>
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label="GitHub 仓库地址"
              placeholder="https://github.com/owner/repo"
              value={repoUrl}
              onValueChange={setRepoUrl}
              startContent={<Search className="h-4 w-4" />}
            />
            <Input
              label="Ref / Branch"
              placeholder="HEAD"
              value={ref}
              onValueChange={setRef}
            />
          </div>

          <div className="mt-3 grid gap-3 md:grid-cols-3">
            <Select
              aria-label="分析深度"
              label="分析深度"
              selectedKeys={new Set([depth])}
              selectionMode="single"
              disallowEmptySelection
              onSelectionChange={(keys) => {
                const value = selectionToString(keys)
                if (value === 'deep' || value === 'standard') {
                  setDepth(value)
                }
              }}
            >
              <SelectItem key="standard">Standard</SelectItem>
              <SelectItem key="deep">Deep</SelectItem>
            </Select>
            <Select
              aria-label="输出语言"
              label="输出语言"
              selectedKeys={new Set([language])}
              selectionMode="single"
              disallowEmptySelection
              onSelectionChange={(keys) => {
                const value = selectionToString(keys)
                if (value === 'zh-CN' || value === 'en') setLanguage(value)
              }}
            >
              <SelectItem key="zh-CN">中文</SelectItem>
              <SelectItem key="en">English</SelectItem>
            </Select>
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
          </div>
          <Select
            aria-label="CLI 工具"
            label="CLI 工具"
            placeholder={selectableTools.length === 0 ? '暂无可用 CLI 工具' : '选择分析工具'}
            description={effectiveCliToolId ? `当前协议：${cliToolProtocolFamilies(toolsById.get(effectiveCliToolId)).join(' / ') || '未知'}` : '沿用 Settings → CLI 工具 中启用的工具'}
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
            description={effectiveModelId ? `默认回退：${effectiveModelId}` : '模型列表来自 Settings → 模型设置 中该 runtime 的配置'}
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

          <div className="mt-3">
            <Textarea
              label="分析特征 / 问题"
              placeholder="每行一个，例如：\n认证流程\n仓库路由与导航\n数据层抽象"
              minRows={4}
              value={featuresText}
              onValueChange={setFeaturesText}
            />
          </div>

          <div className="mt-4 flex items-center justify-end gap-2">
            <Button variant="light" onPress={() => {
              setRepoUrl('')
              setRef('HEAD')
              setFeaturesText('认证流程\n仓库路由与导航')
              setDepth('standard')
              setLanguage('zh-CN')
              setAnalyzerMode('full')
              setSelectedCliToolId('')
              setSelectedModelId('')
            }}>
              重置
            </Button>
            <Button color="primary" startContent={<Plus className="h-4 w-4" />} isLoading={submitting} onPress={() => void onSubmit()}>
              创建分析
            </Button>
          </div>
        </div>

        {createdAnalysis ? (
          <div className="rounded-2xl border border-success/40 bg-success/5 p-4">
            <div className="mb-2 flex items-center gap-2 text-success">
              <Sparkles className="h-4 w-4" />
              <span className="font-medium">最近创建的分析</span>
            </div>
            <div className="grid gap-2 text-sm md:grid-cols-2">
              <div>
                <span className="text-muted-foreground">分析 ID：</span>
                <span className="font-mono">{createdAnalysis.analysis_id}</span>
              </div>
              <div>
                <span className="text-muted-foreground">执行 ID：</span>
                <span className="font-mono">{createdAnalysis.execution_id || '-'}</span>
              </div>
              <div>
                <span className="text-muted-foreground">状态：</span>
                <Chip size="sm" color={analysisStatusColor(createdAnalysis.status)} variant="flat">
                  {formatAnalysisStatus(createdAnalysis.status)}
                </Chip>
              </div>
              <div>
                <span className="text-muted-foreground">创建时间：</span>
                <span>{createdAnalysis.created_at ? formatRelativeTime(createdAnalysis.created_at) : '-'}</span>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {createdRepositoryId ? (
                <Button size="sm" variant="flat" onPress={() => goToRepoLibraryRepository(createdRepositoryId)}>
                  打开仓库详情
                </Button>
              ) : null}
              <Button size="sm" variant="light" onPress={() => goToRepoLibraryPatternSearch()}>
                打开 Pattern Search
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </RepoLibraryShell>
  )
}
