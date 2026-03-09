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
  type CLITool,
  type LLMModelProfile,
  type RepoLibraryAnalysisRequest,
  type RepoLibraryAnalysisRun,
  type RepoLibraryCreateAnalysisResponse,
  type RepoLibraryDepth,
} from '@/lib/daemon'
import { buildCLIToolModelProfiles, cliToolDefaultModelID } from '@/lib/cliToolModels'
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
  const [toolModels, setToolModels] = useState<LLMModelProfile[]>([])
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
              subtitle={item.repo_url}
              meta={formatRelativeTime(item.created_at || item.updated_at || 0)}
              active={Boolean(createdRepositoryId && item.repository_id === createdRepositoryId)}
              onPress={() => goToRepoLibraryRepository(item.repository_id)}
            />
          ))}
        </div>
      )}
      <LoadingVeil visible={loading && hasRepositoryCache} compact label="正在刷新仓库列表…" />
    </div>
  )

  return (
    <RepoLibraryShell
      title="Github 知识库"
      headerMeta={
        <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          <span>Repo Library</span>
          <span>·</span>
          <span>{items.length} 个仓库</span>
        </div>
      }
      headerActions={
        <>
          <Button variant="flat" size="sm" startContent={<Search className="h-4 w-4" />} onPress={goToRepoLibraryPatternSearch}>
            知识库检索
          </Button>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh({ force: true })}>
            刷新列表
          </Button>
        </>
      }
      sidebarTitle="仓库"
      sidebarCount={items.length}
      sidebarAction={
        <Button
          color="primary"
          size="sm"
          className="w-[25%] min-w-[86px] rounded-2xl"
          startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />}
          onPress={() => {
            document.getElementById('repo-library-analyze-form')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
            window.requestAnimationFrame(() => {
              const input = document.getElementById('repo-library-repo-url') as HTMLInputElement | null
              input?.focus()
            })
          }}
        >
          添加仓库
        </Button>
      }
      sidebarContent={sidebarContent}
    >
      <div className="relative">
        {error && hasRepositoryCache ? <Alert color="danger" title="刷新失败，已保留上次内容" description={error} className="mb-4" /> : null}
        <section id="repo-library-analyze-form" className="rounded-2xl border bg-card p-5 shadow-sm">
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
            id="repo-library-repo-url"
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
              <Button variant="flat" size="sm" onPress={() => goToRepoLibraryRepository(createdRepositoryId)}>
                打开仓库详情
              </Button>
            ) : null}
          </div>
        </section>
      ) : null}
      <LoadingVeil visible={loading && hasRepositoryCache} label="正在同步知识库内容…" />
      </div>
    </RepoLibraryShell>
  )
}
