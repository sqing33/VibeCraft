import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Chip,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Select,
  SelectItem,
  Skeleton,
  Textarea,
} from "@heroui/react";
import { Bot, History, Plus, RefreshCw, Sparkles, Trash2, WandSparkles } from "lucide-react";

import { formatRelativeTime } from "@/lib/time";
import { toast } from "@/lib/toast";
import {
  fetchExpertBuilderSession,
  fetchExpertBuilderSessions,
  fetchExpertSettings,
  fetchExperts,
  postExpertBuilderMessage,
  postExpertBuilderPublish,
  postExpertBuilderSession,
  putExpertSettings,
  type ExpertBuilderSession,
  type ExpertBuilderSessionDetail,
  type ExpertBuilderSnapshot,
  type ExpertSettings,
  type ExpertSettingsItem,
} from "@/lib/daemon";
import { useDaemonStore } from "@/stores/daemonStore";

function selectionToString(keys: "all" | Set<string | number>): string {
  if (keys === "all") return "";
  for (const key of keys) return String(key);
  return "";
}

function formatManagedSource(source?: string): string {
  switch ((source ?? "").trim()) {
    case "builtin":
      return "内建";
    case "llm-model":
      return "模型镜像";
    case "expert-profile":
      return "自定义专家";
    default:
      return source?.trim() || "未知";
  }
}

function formatCategory(category?: string): string {
  switch ((category ?? "").trim()) {
    case "design":
      return "设计";
    case "planning":
      return "规划";
    case "ops":
      return "运维";
    case "coding":
      return "开发";
    case "research":
      return "研究";
    case "general":
      return "通用";
    default:
      return category?.trim() || "未分类";
  }
}

function formatProvider(provider?: string): string {
  switch ((provider ?? "").trim()) {
    case "openai":
      return "OpenAI";
    case "anthropic":
      return "Anthropic";
    case "demo":
      return "演示";
    case "process":
      return "本地进程";
    case "cli":
      return "CLI";
    default:
      return provider?.trim() || "未知";
  }
}

function formatFallback(expert: ExpertSettingsItem | null): string {
  if (!expert?.secondary_model_id) return "无副模型";
  return expert.fallback_on && expert.fallback_on.length > 0
    ? `失败时切换：${expert.fallback_on.join(" / ")}`
    : "失败时切换副模型";
}

function toPutExperts(experts: ExpertSettingsItem[]) {
  return {
    experts: experts
      .filter((expert) => expert.editable)
      .map((expert) => ({
        id: expert.id,
        label: expert.label,
        description: expert.description,
        category: expert.category,
        avatar: expert.avatar,
        primary_model_id: expert.primary_model_id,
        secondary_model_id: expert.secondary_model_id,
        fallback_on: expert.fallback_on ?? [],
        enabled_skills: expert.enabled_skills ?? [],
        system_prompt: expert.system_prompt,
        prompt_template: expert.prompt_template,
        output_format: expert.output_format,
        max_output_tokens: expert.max_output_tokens,
        temperature: expert.temperature,
        timeout_ms: expert.timeout_ms,
        builder_expert_id: expert.builder_expert_id,
        builder_session_id: expert.builder_session_id,
        builder_snapshot_id: expert.builder_snapshot_id,
        generated_by: expert.generated_by,
        generated_at: expert.generated_at,
        updated_at: expert.updated_at,
        enabled: expert.enabled,
      })),
  };
}

export function ExpertSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const setExperts = useDaemonStore((s) => s.setExperts);
  const setExpertsError = useDaemonStore((s) => s.setExpertsError);

  const [data, setData] = useState<ExpertSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const [workbenchOpen, setWorkbenchOpen] = useState(false);
  const [builderModelId, setBuilderModelId] = useState("");
  const [sessionList, setSessionList] = useState<ExpertBuilderSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [sessionDetail, setSessionDetail] = useState<ExpertBuilderSessionDetail | null>(null);
  const [selectedSnapshotId, setSelectedSnapshotId] = useState<string | null>(null);
  const [sessionLoading, setSessionLoading] = useState(false);
  const [sessionSending, setSessionSending] = useState(false);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const [builderInput, setBuilderInput] = useState("");

  const experts = useMemo(() => data?.experts ?? [], [data]);
  const skills = useMemo(() => data?.skills ?? [], [data]);
  const modelOptions = useMemo(
    () =>
      experts
        .filter((expert) => (expert.managed_source ?? "") === "llm-model")
        .map((expert) => ({
          id: expert.id,
          label: expert.label || expert.id,
          provider: expert.provider || "",
          model: expert.model || "",
        })),
    [experts],
  );

  const selectedExpert = useMemo(() => {
    if (!experts.length) return null;
    return experts.find((expert) => expert.id === selectedId) ?? experts[0] ?? null;
  }, [experts, selectedId]);

  const activeSnapshot: ExpertBuilderSnapshot | null = useMemo(() => {
    const snapshots = sessionDetail?.snapshots ?? [];
    if (snapshots.length === 0) return null;
    if (!selectedSnapshotId) return snapshots[0] ?? null;
    return snapshots.find((snapshot) => snapshot.id === selectedSnapshotId) ?? snapshots[0] ?? null;
  }, [selectedSnapshotId, sessionDetail]);

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchExpertSettings(daemonUrl);
      setData(res);
      setSelectedId((prev) => (prev && res.experts.some((expert) => expert.id === prev) ? prev : (res.experts[0]?.id ?? null)));
      setBuilderModelId((prev) => prev || res.experts.find((expert) => (expert.managed_source ?? "") === "llm-model")?.id || "");
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      toast({ variant: "destructive", title: "加载专家设置失败", description: message });
    } finally {
      setLoading(false);
    }
  }, [daemonUrl]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const refreshPublicExperts = useCallback(async () => {
    try {
      const publicExperts = await fetchExperts(daemonUrl);
      setExperts(publicExperts);
      setExpertsError(null);
    } catch (err: unknown) {
      setExpertsError(err instanceof Error ? err.message : String(err));
    }
  }, [daemonUrl, setExperts, setExpertsError]);

  const saveExperts = useCallback(async (nextExperts: ExpertSettingsItem[]) => {
    setSaving(true);
    try {
      const res = await putExpertSettings(daemonUrl, toPutExperts(nextExperts));
      setData(res);
      setSelectedId((prev) => (prev && res.experts.some((expert) => expert.id === prev) ? prev : (res.experts[0]?.id ?? null)));
      await refreshPublicExperts();
      toast({ title: "专家设置已保存" });
      return res;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      toast({ variant: "destructive", title: "保存专家失败", description: message });
      throw err;
    } finally {
      setSaving(false);
    }
  }, [daemonUrl, refreshPublicExperts]);

  const customExperts = useMemo(() => experts.filter((expert) => expert.editable), [experts]);

  const onToggleEnabled = useCallback(async () => {
    if (!selectedExpert?.editable) return;
    const next = customExperts.map((expert) => expert.id === selectedExpert.id ? { ...expert, enabled: !expert.enabled } : expert);
    await saveExperts(next);
  }, [customExperts, saveExperts, selectedExpert]);

  const onDelete = useCallback(async () => {
    if (!selectedExpert?.editable) return;
    if (!window.confirm(`确定删除专家 ${selectedExpert.label || selectedExpert.id} 吗？`)) return;
    const next = customExperts.filter((expert) => expert.id !== selectedExpert.id);
    await saveExperts(next);
  }, [customExperts, saveExperts, selectedExpert]);

  const loadSessions = useCallback(async (targetExpertId?: string) => {
    const res = await fetchExpertBuilderSessions(daemonUrl, { targetExpertId, limit: 30 });
    setSessionList(res.sessions);
    return res.sessions;
  }, [daemonUrl]);

  const loadSessionDetail = useCallback(async (sessionId: string) => {
    setSessionLoading(true);
    setSessionError(null);
    try {
      const detail = await fetchExpertBuilderSession(daemonUrl, sessionId);
      setSessionDetail(detail);
      setActiveSessionId(detail.session.id);
      setBuilderModelId(detail.session.builder_model_id);
      setSelectedSnapshotId(detail.session.latest_snapshot_id ?? detail.snapshots[0]?.id ?? null);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setSessionError(message);
    } finally {
      setSessionLoading(false);
    }
  }, [daemonUrl]);

  const openWorkbench = useCallback(async (targetExpert?: ExpertSettingsItem | null) => {
    setWorkbenchOpen(true);
    setSessionError(null);
    setSessionDetail(null);
    setActiveSessionId(null);
    setSelectedSnapshotId(null);
    setBuilderInput("");
    const sessions = await loadSessions(targetExpert?.id);
    const preferredSessionId = targetExpert
      ? (targetExpert.builder_session_id || sessions.find((session) => session.target_expert_id === targetExpert.id)?.id || null)
      : null;
    if (preferredSessionId) {
      await loadSessionDetail(preferredSessionId);
    }
  }, [loadSessionDetail, loadSessions]);

  const ensureSession = useCallback(async () => {
    if (activeSessionId) return activeSessionId;
    if (!builderModelId) throw new Error("请先选择生成模型。");
    const targetExpertId = selectedExpert?.editable ? selectedExpert.id : undefined;
    const title = targetExpertId ? `优化 ${selectedExpert?.label || targetExpertId}` : "专家设计会话";
    const created = await postExpertBuilderSession(daemonUrl, { title, target_expert_id: targetExpertId, builder_model_id: builderModelId });
    const nextSessions = await loadSessions(targetExpertId);
    setSessionList(nextSessions);
    setActiveSessionId(created.session.id);
    setBuilderModelId(created.session.builder_model_id);
    return created.session.id;
  }, [activeSessionId, builderModelId, daemonUrl, loadSessions, selectedExpert]);

  const onSendMessage = useCallback(async () => {
    const content = builderInput.trim();
    if (!content) return;
    setSessionSending(true);
    setSessionError(null);
    try {
      const sessionId = await ensureSession();
      const detail = await postExpertBuilderMessage(daemonUrl, sessionId, { content });
      setBuilderInput("");
      setSessionDetail(detail);
      setSelectedSnapshotId(detail.session.latest_snapshot_id ?? detail.snapshots[0]?.id ?? null);
      const nextSessions = await loadSessions(selectedExpert?.id);
      setSessionList(nextSessions);
    } catch (err: unknown) {
      setSessionError(err instanceof Error ? err.message : String(err));
    } finally {
      setSessionSending(false);
    }
  }, [builderInput, daemonUrl, ensureSession, loadSessions, selectedExpert?.id]);

  const onPublishSnapshot = useCallback(async () => {
    if (!activeSessionId) return;
    try {
      const result = await postExpertBuilderPublish(daemonUrl, activeSessionId, {
        snapshot_id: selectedSnapshotId || undefined,
        expert_id: selectedExpert?.editable ? selectedExpert.id : undefined,
      });
      toast({ title: "专家已发布", description: result.published_expert.label || result.published_expert.id });
      await loadSettings();
      const nextSessions = await loadSessions(selectedExpert?.id);
      setSessionList(nextSessions);
      await loadSessionDetail(activeSessionId);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setSessionError(message);
      toast({ variant: "destructive", title: "发布专家失败", description: message });
    }
  }, [activeSessionId, daemonUrl, loadSessionDetail, loadSessions, loadSettings, selectedExpert, selectedSnapshotId]);

  if (loading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-lg" />
        <Skeleton className="h-96 w-full rounded-lg" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <section className="grid gap-3 rounded-lg border bg-card p-4 sm:grid-cols-4">
        <div>
          <div className="text-xs text-muted-foreground">专家总数</div>
          <div className="mt-1 text-2xl font-semibold">{experts.length}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">自定义专家</div>
          <div className="mt-1 text-2xl font-semibold">{customExperts.length}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">可用生成模型</div>
          <div className="mt-1 text-2xl font-semibold">{modelOptions.length}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">发现技能</div>
          <div className="mt-1 text-2xl font-semibold">{skills.length}</div>
        </div>
      </section>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap gap-2">
          <Chip variant="flat">只读系统专家：{experts.filter((expert) => !expert.editable).length}</Chip>
          <Chip variant="flat" color="primary">可编辑专家：{customExperts.length}</Chip>
        </div>
        <div className="flex gap-2">
          <Button variant="flat" startContent={<RefreshCw className="h-4 w-4" />} onPress={() => void loadSettings()} isDisabled={saving}>
            重新加载
          </Button>
          <Button color="primary" startContent={<Plus className="h-4 w-4" />} onPress={() => void openWorkbench(null)}>
            AI 创建专家
          </Button>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
        <section className="space-y-3">
          {experts.map((expert) => {
            const active = selectedExpert?.id === expert.id;
            return (
              <button key={expert.id} type="button" className={`w-full rounded-lg border p-3 text-left transition ${active ? "border-primary bg-primary/5" : "bg-card hover:bg-muted/40"}`} onClick={() => setSelectedId(expert.id)}>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold">{expert.avatar ? `${expert.avatar} ` : ""}{expert.label || expert.id}</div>
                    <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{expert.description || "暂无描述"}</div>
                  </div>
                  <Chip size="sm" variant="flat" color={expert.editable ? "primary" : "default"}>{formatManagedSource(expert.managed_source)}</Chip>
                </div>
                <div className="mt-3 flex flex-wrap gap-1">
                  <Chip size="sm" variant="bordered">{formatProvider(expert.provider)}</Chip>
                  <Chip size="sm" variant="bordered">主：{expert.primary_model_id || expert.model || "-"}</Chip>
                  {expert.secondary_model_id ? <Chip size="sm" variant="bordered">副：{expert.secondary_model_id}</Chip> : null}
                  {!expert.enabled ? <Chip size="sm" color="warning" variant="flat">已停用</Chip> : null}
                </div>
              </button>
            );
          })}
        </section>

        <section className="rounded-lg border bg-card p-4">
          {selectedExpert ? (
            <div className="space-y-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="text-xl font-semibold">{selectedExpert.avatar ? `${selectedExpert.avatar} ` : ""}{selectedExpert.label || selectedExpert.id}</div>
                  <div className="mt-1 text-sm text-muted-foreground">{selectedExpert.description || "暂无描述"}</div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Chip variant="flat">{formatManagedSource(selectedExpert.managed_source)}</Chip>
                  <Chip variant="flat" color={selectedExpert.enabled ? "success" : "warning"}>{selectedExpert.enabled ? "已启用" : "已停用"}</Chip>
                  {selectedExpert.editable ? <Chip variant="flat" color="primary">可编辑</Chip> : <Chip variant="flat">只读</Chip>}
                </div>
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-lg border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">分类</div>
                  <div className="mt-1 text-sm font-medium">{formatCategory(selectedExpert.category)}</div>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">模型策略</div>
                  <div className="mt-1 text-sm font-medium">主：{selectedExpert.primary_model_id || selectedExpert.model || "-"}</div>
                  <div className="text-xs text-muted-foreground">副：{selectedExpert.secondary_model_id || "未配置"}</div>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3 sm:col-span-2">
                  <div className="text-xs text-muted-foreground">回退策略</div>
                  <div className="mt-1 text-sm font-medium">{formatFallback(selectedExpert)}</div>
                </div>
              </div>

              <div className="space-y-2">
                <div className="text-sm font-medium">启用技能</div>
                <div className="flex flex-wrap gap-2">
                  {(selectedExpert.enabled_skills ?? []).length > 0 ? (selectedExpert.enabled_skills ?? []).map((skill) => <Chip key={skill} size="sm" variant="flat">{skill}</Chip>) : <div className="text-sm text-muted-foreground">未配置专属技能</div>}
                </div>
              </div>

              <div className="space-y-2">
                <div className="text-sm font-medium">系统提示词摘要</div>
                <Textarea isReadOnly minRows={6} value={selectedExpert.system_prompt || "暂无系统提示词"} />
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-lg border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">Prompt 模板</div>
                  <div className="mt-1 whitespace-pre-wrap break-words text-sm">{selectedExpert.prompt_template || "{{prompt}}"}</div>
                </div>
                <div className="rounded-lg border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">输出格式</div>
                  <div className="mt-1 whitespace-pre-wrap break-words text-sm">{selectedExpert.output_format || "未指定"}</div>
                </div>
              </div>

              <div className="grid gap-3 sm:grid-cols-3">
                <div className="rounded-lg border bg-muted/20 p-3"><div className="text-xs text-muted-foreground">Max Tokens</div><div className="mt-1 text-sm">{selectedExpert.max_output_tokens || 0}</div></div>
                <div className="rounded-lg border bg-muted/20 p-3"><div className="text-xs text-muted-foreground">Temperature</div><div className="mt-1 text-sm">{selectedExpert.temperature ?? 0}</div></div>
                <div className="rounded-lg border bg-muted/20 p-3"><div className="text-xs text-muted-foreground">Timeout</div><div className="mt-1 text-sm">{selectedExpert.timeout_ms || 0} ms</div></div>
              </div>

              <div className="rounded-lg border bg-muted/20 p-3 text-sm text-muted-foreground">
                生成来源：{selectedExpert.generated_by || "手动/系统"}
                {selectedExpert.builder_expert_id ? ` · 生成模型：${selectedExpert.builder_expert_id}` : ""}
                {selectedExpert.builder_session_id ? ` · 会话：${selectedExpert.builder_session_id}` : ""}
              </div>

              <div className="flex flex-wrap gap-2">
                <Button color="primary" variant="flat" startContent={<WandSparkles className="h-4 w-4" />} onPress={() => void openWorkbench(selectedExpert)}>
                  {selectedExpert.builder_session_id ? "继续优化" : "创建优化会话"}
                </Button>
                {selectedExpert.editable ? (
                  <>
                    <Button color={selectedExpert.enabled ? "warning" : "success"} variant="flat" onPress={() => void onToggleEnabled()} isDisabled={saving}>
                      {selectedExpert.enabled ? "停用专家" : "启用专家"}
                    </Button>
                    <Button color="danger" variant="flat" startContent={<Trash2 className="h-4 w-4" />} onPress={() => void onDelete()} isDisabled={saving}>
                      删除专家
                    </Button>
                  </>
                ) : null}
              </div>
            </div>
          ) : <div className="text-sm text-muted-foreground">暂无专家。</div>}
        </section>
      </div>

      <Modal isOpen={workbenchOpen} onOpenChange={setWorkbenchOpen} size="5xl" scrollBehavior="inside">
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex items-center gap-2"><Sparkles className="h-5 w-5" /> 专家生成工作台</ModalHeader>
              <ModalBody>
                <div className="grid gap-4 xl:grid-cols-[260px_minmax(0,1fr)_360px]">
                  <div className="space-y-4 rounded-lg border bg-card p-4">
                    <div className="space-y-3">
                      <Select aria-label="生成模型" label="生成模型" selectionMode="single" disallowEmptySelection selectedKeys={builderModelId ? new Set([builderModelId]) : new Set([])} onSelectionChange={(keys) => setBuilderModelId(selectionToString(keys))}>
                        {modelOptions.map((model) => <SelectItem key={model.id}>{model.label} · {formatProvider(model.provider)}/{model.model}</SelectItem>)}
                      </Select>
                      <Button color="primary" variant="flat" startContent={<Plus className="h-4 w-4" />} onPress={() => { setActiveSessionId(null); setSessionDetail(null); setSelectedSnapshotId(null); }}>
                        新建会话（下一条消息创建）
                      </Button>
                    </div>
                    <div className="space-y-2">
                      <div className="flex items-center gap-2 text-sm font-medium"><History className="h-4 w-4" /> 历史会话</div>
                      <div className="max-h-[420px] space-y-2 overflow-y-auto pr-1">
                        {sessionList.length === 0 ? <div className="text-sm text-muted-foreground">暂无历史会话</div> : null}
                        {sessionList.map((session) => (
                          <button key={session.id} type="button" className={`w-full rounded-lg border p-3 text-left ${activeSessionId === session.id ? "border-primary bg-primary/5" : "hover:bg-muted/30"}`} onClick={() => void loadSessionDetail(session.id)}>
                            <div className="text-sm font-medium line-clamp-2">{session.title}</div>
                            <div className="mt-1 text-xs text-muted-foreground">模型：{session.builder_model_id}</div>
                            <div className="text-xs text-muted-foreground">更新于 {formatRelativeTime(session.updated_at)}</div>
                          </button>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-2">
                      <div className="text-sm font-medium">快照历史</div>
                      <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
                        {(sessionDetail?.snapshots ?? []).length === 0 ? <div className="text-sm text-muted-foreground">暂无快照</div> : null}
                        {(sessionDetail?.snapshots ?? []).map((snapshot) => (
                          <button key={snapshot.id} type="button" className={`w-full rounded-lg border p-2 text-left ${selectedSnapshotId === snapshot.id ? "border-primary bg-primary/5" : "hover:bg-muted/30"}`} onClick={() => setSelectedSnapshotId(snapshot.id)}>
                            <div className="text-sm font-medium">版本 v{snapshot.version}</div>
                            <div className="text-xs text-muted-foreground">{formatRelativeTime(snapshot.created_at)}</div>
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>

                  <div className="space-y-4 rounded-lg border bg-card p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div>
                        <div className="text-sm font-medium">对话历史</div>
                        <div className="text-xs text-muted-foreground">长对话会保存在 SQLite 中，后续可以继续微调。</div>
                      </div>
                      {sessionDetail?.session ? <Chip variant="flat">当前会话：{sessionDetail.session.title}</Chip> : null}
                    </div>
                    {sessionError ? <Alert color="danger" title="操作失败" description={sessionError} /> : null}
                    <div className="max-h-[520px] space-y-3 overflow-y-auto pr-1">
                      {sessionLoading ? <Skeleton className="h-40 w-full rounded-lg" /> : null}
                      {!sessionLoading && (sessionDetail?.messages ?? []).length === 0 ? (
                        <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">你可以和 AI 持续对话，描述需求、指出不足、要求重做某个字段。每一轮都会保存历史与新快照。</div>
                      ) : null}
                      {(sessionDetail?.messages ?? []).map((message) => (
                        <div key={message.id} className={`rounded-lg p-3 text-sm ${message.role === "assistant" ? "bg-primary/5" : "bg-background border"}`}>
                          <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">{message.role === "assistant" ? <Bot className="h-3.5 w-3.5" /> : null}{message.role === "assistant" ? "AI" : "你"} · {formatRelativeTime(message.created_at)}</div>
                          <div className="whitespace-pre-wrap break-words">{message.content_text}</div>
                        </div>
                      ))}
                    </div>
                    <Textarea label="继续描述需求" minRows={6} value={builderInput} onValueChange={setBuilderInput} placeholder="例如：这个专家还不够懂 B 端信息层级，请加强表格、筛选和交互流程方面的能力。" />
                  </div>

                  <div className="space-y-3 rounded-lg border bg-card p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div className="text-sm font-medium">当前草稿预览</div>
                      {activeSnapshot ? <Chip color="primary" variant="flat">v{activeSnapshot.version}</Chip> : null}
                    </div>
                    {activeSnapshot ? (
                      <>
                        <Input isReadOnly label="ID" value={activeSnapshot.draft.id} />
                        <Input isReadOnly label="名称" value={activeSnapshot.draft.label} />
                        <Textarea isReadOnly label="描述" minRows={3} value={activeSnapshot.draft.description || ""} />
                        <div className="grid gap-2 sm:grid-cols-2">
                          <Input isReadOnly label="主模型" value={activeSnapshot.draft.primary_model_id || ""} />
                          <Input isReadOnly label="副模型" value={activeSnapshot.draft.secondary_model_id || ""} />
                        </div>
                        <div className="flex flex-wrap gap-1">
                          {(activeSnapshot.draft.enabled_skills ?? []).length > 0 ? (activeSnapshot.draft.enabled_skills ?? []).map((skill) => <Chip key={skill} size="sm" variant="flat">{skill}</Chip>) : <div className="text-sm text-muted-foreground">暂无技能</div>}
                        </div>
                        <Textarea isReadOnly label="系统提示词" minRows={8} value={activeSnapshot.draft.system_prompt || ""} />
                        {activeSnapshot.warnings && activeSnapshot.warnings.length > 0 ? <Alert color="warning" title="生成提醒" description={activeSnapshot.warnings.join("；")} /> : null}
                      </>
                    ) : (
                      <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">发送第一条消息后，这里会出现当前草稿与快照版本。</div>
                    )}
                  </div>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button variant="flat" onPress={() => setWorkbenchOpen(false)}>关闭</Button>
                <Button color="primary" variant="flat" onPress={() => void onSendMessage()} isLoading={sessionSending}>{activeSessionId ? "继续生成" : "开始生成"}</Button>
                <Button color="primary" onPress={() => void onPublishSnapshot()} isDisabled={!activeSessionId || !activeSnapshot || saving}>发布当前快照</Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
}
