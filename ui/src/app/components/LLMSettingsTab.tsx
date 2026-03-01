import { useCallback, useEffect, useMemo, useState } from "react";
import { Play, Plus, RefreshCw, Save, Trash2 } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/use-toast";
import {
  fetchExperts,
  fetchLLMSettings,
  postLLMTest,
  putLLMSettings,
  type LLMModelProfile,
  type LLMSettings,
  type LLMSource,
  type PutLLMSettingsRequest,
} from "@/lib/daemon";
import { useDaemonStore } from "@/stores/daemonStore";

type SourceDraft = {
  local_id: string;
  id: string;
  label: string;
  label_touched: boolean;
  base_url: string;
  has_key: boolean;
  masked_key: string;
  key_input: string;
  key_touched: boolean;
};

type ModelDraft = {
  local_id: string;
  id: string;
  label: string;
  provider: string;
  model: string;
  source_id: string;
};

function normalizeBaseUrl(raw: string): string {
  const url = (raw ?? "").trim();
  if (!url) return "";
  return url.endsWith("/") ? url.slice(0, -1) : url;
}

function newLocalID(): string {
  return `${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function toDraftSources(list: LLMSource[]): SourceDraft[] {
  return list.map((s) => ({
    local_id: newLocalID(),
    id: (s.id ?? "").trim(),
    label: (s.label ?? "").trim(),
    label_touched: false,
    base_url: normalizeBaseUrl(String(s.base_url ?? "")),
    has_key: Boolean(s.has_key),
    masked_key: String(s.masked_key ?? ""),
    key_input: "",
    key_touched: false,
  }));
}

function toDraftModels(list: LLMModelProfile[]): ModelDraft[] {
  return list.map((m) => ({
    local_id: newLocalID(),
    id: (m.id ?? "").trim(),
    label: (m.label ?? "").trim(),
    provider: (m.provider ?? "").trim(),
    model: (m.model ?? "").trim(),
    source_id: (m.source_id ?? "").trim(),
  }));
}

function buildPutRequest(
  sources: SourceDraft[],
  models: ModelDraft[],
): PutLLMSettingsRequest {
  return {
    sources: sources.map((s) => ({
      id: s.id.trim(),
      label: s.label.trim(),
      provider: "",
      base_url: normalizeBaseUrl(s.base_url),
      ...(s.key_touched ? { api_key: s.key_input.trim() } : {}),
    })),
    models: models.map((m) => ({
      id: m.id.trim(),
      label: m.label.trim(),
      provider: m.provider.trim(),
      model: m.model.trim(),
      source_id: m.source_id.trim(),
    })),
  };
}

function nextID(prefix: string, used: Set<string>): string {
  if (!used.has(prefix)) return prefix;
  for (let i = 2; i < 1000; i += 1) {
    const id = `${prefix}-${i}`;
    if (!used.has(id)) return id;
  }
  return `${prefix}-${Date.now()}`;
}

export function LLMSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const setExperts = useDaemonStore((s) => s.setExperts);
  const setExpertsError = useDaemonStore((s) => s.setExpertsError);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [sources, setSources] = useState<SourceDraft[]>([]);
  const [models, setModels] = useState<ModelDraft[]>([]);
  const [testingId, setTestingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const res = await fetchLLMSettings(daemonUrl);
      setSources(toDraftSources(res.sources ?? []));
      setModels(toDraftModels(res.models ?? []));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [daemonUrl]);

  useEffect(() => {
    void load();
  }, [load]);

  const sourceOptions = useMemo(() => {
    return sources.map((s) => ({
      id: s.id,
      label: (s.label || s.id).trim(),
    }));
  }, [sources]);

  const formatSourceOption = useCallback(
    (id: string) => {
      const s = sources.find((x) => x.id === id);
      if (!s) return id || "未选择";
      const label = (s.label || s.id).trim();
      const sid = (s.id || "").trim();
      if (!label || label === sid) return sid || "未命名";
      return `${label} (${sid})`;
    },
    [sources],
  );

  const onAddSource = () => {
    const used = new Set(sources.map((s) => s.id));
    const id = nextID("source", used);
    setSources((prev) => [
      ...prev,
      {
        local_id: newLocalID(),
        id,
        label: id,
        label_touched: false,
        base_url: "",
        has_key: false,
        masked_key: "",
        key_input: "",
        key_touched: true,
      },
    ]);
  };

  const onDeleteSource = (id: string) => {
    const usedBy = models.filter((m) => m.source_id === id);
    if (usedBy.length > 0) {
      toast({
        variant: "destructive",
        title: "无法删除 Source",
        description: `仍有 ${usedBy.length} 个模型在使用该 Source。`,
      });
      return;
    }
    setSources((prev) => prev.filter((s) => s.id !== id));
  };

  const onAddModel = () => {
    const used = new Set(models.map((m) => m.id));
    const provider = "openai";
    const source_id = sourceOptions[0]?.id ?? "";
    const id = nextID("codex", used);
    setModels((prev) => [
      ...prev,
      { local_id: newLocalID(), id, label: id, provider, model: "", source_id },
    ]);
  };

  const onDeleteModel = (id: string) => {
    setModels((prev) => prev.filter((m) => m.id !== id));
  };

  const onSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const req = buildPutRequest(sources, models);
      const saved: LLMSettings = await putLLMSettings(daemonUrl, req);
      setSources(toDraftSources(saved.sources ?? []));
      setModels(toDraftModels(saved.models ?? []));

      const experts = await fetchExperts(daemonUrl);
      setExperts(experts);
      setExpertsError(null);

      toast({ title: "模型设置已保存" });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      toast({
        variant: "destructive",
        title: "保存失败",
        description: message,
      });
    } finally {
      setSaving(false);
    }
  };

  const onTestModel = async (m: ModelDraft) => {
    if (testingId) return;

    const provider = (m.provider ?? "").trim();
    const model = (m.model ?? "").trim();
    const sourceID = (m.source_id ?? "").trim();
    if (!provider || !model || !sourceID) {
      toast({
        variant: "destructive",
        title: "配置不完整",
        description: "请先填写 SDK、Source 与模型名。",
      });
      return;
    }

    const src = sources.find((s) => s.id === sourceID);
    if (!src) {
      toast({
        variant: "destructive",
        title: "Source 不存在",
        description: `找不到 Source：${sourceID}`,
      });
      return;
    }

    const apiKey = (src.key_input ?? "").trim();
    if (!apiKey && !src.has_key) {
      toast({
        variant: "destructive",
        title: "缺少 API Key",
        description: "请先为该 Source 填写 API Key（或先保存已有 Key）。",
      });
      return;
    }

    setTestingId(m.id);
    try {
      const res = await postLLMTest(daemonUrl, {
        provider,
        model,
        source_id: src.id,
        base_url: normalizeBaseUrl(src.base_url),
        api_key: apiKey ? apiKey : undefined,
        prompt: "Reply with a single word: OK",
      });

      toast({
        title: "测试成功",
        description: `${res.output || "OK"}（${res.latency_ms}ms）`,
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      toast({
        variant: "destructive",
        title: "测试失败",
        description: message,
      });
    } finally {
      setTestingId(null);
    }
  };

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {error ? (
        <Alert variant="destructive">
          <AlertTitle>加载/保存失败</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium">API 源</div>
          <Button variant="secondary" size="sm" onClick={onAddSource}>
            <Plus className="mr-2 h-4 w-4" />
            添加来源
          </Button>
        </div>

        <div className="space-y-3">
          {sources.length === 0 ? (
            <div className="text-sm text-muted-foreground">
              尚未配置 Source。
            </div>
          ) : null}

          {sources.map((s) => (
            <div key={s.local_id} className="rounded-lg border bg-card p-3">
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="truncate text-sm font-semibold">
                    {(s.label || "").trim() || "未命名"}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2">
                    {s.has_key ? (
                      <Badge variant="outline">
                        Key：{s.masked_key || "已设置"}
                      </Badge>
                    ) : (
                      <Badge variant="outline">Key：未设置</Badge>
                    )}
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onDeleteSource(s.id)}
                  aria-label="删除 Source"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>

              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">ID</div>
                  <Input
                    value={s.id}
                    onChange={(e) =>
                      setSources((prev) => {
                        const nextID = e.target.value;
                        const old = prev.find((x) => x.local_id === s.local_id);
                        const oldID = (old?.id ?? "").trim();

                        // 级联更新：source_id 改名时同步 models 引用。
                        if (oldID && oldID !== nextID) {
                          setModels((ms) =>
                            ms.map((m) =>
                              m.source_id === oldID
                                ? { ...m, source_id: nextID }
                                : m,
                            ),
                          );
                        }

                        return prev.map((x) => {
                          if (x.local_id !== s.local_id) return x;
                          return {
                            ...x,
                            id: nextID,
                          };
                        });
                      })
                    }
                    placeholder="source"
                  />
                </div>

                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">名称</div>
                  <Input
                    value={s.label}
                    onChange={(e) =>
                      setSources((prev) =>
                        prev.map((x) =>
                          x.local_id === s.local_id
                            ? {
                                ...x,
                                label: e.target.value,
                                label_touched: true,
                              }
                            : x,
                        ),
                      )
                    }
                    placeholder="自定义源"
                  />
                </div>

                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">
                    Base URL（可选，不填使用官方接口）
                  </div>
                  <Input
                    value={s.base_url}
                    onChange={(e) =>
                      setSources((prev) =>
                        prev.map((x) =>
                          x.local_id === s.local_id
                            ? { ...x, base_url: e.target.value }
                            : x,
                        ),
                      )
                    }
                    placeholder="https://api.example.com"
                  />
                </div>

                <div className="grid gap-2 sm:col-span-2">
                  <div className="text-xs text-muted-foreground">
                    API Key（留空表示不修改；空字符串表示清空）
                  </div>
                  <Input
                    type="password"
                    value={s.key_input}
                    onChange={(e) =>
                      setSources((prev) =>
                        prev.map((x) =>
                          x.local_id === s.local_id
                            ? {
                                ...x,
                                key_input: e.target.value,
                                key_touched: true,
                              }
                            : x,
                        ),
                      )
                    }
                    placeholder={s.has_key ? "留空不修改" : "sk-..."}
                    autoComplete="off"
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium">模型</div>
          <Button variant="secondary" size="sm" onClick={onAddModel}>
            <Plus className="mr-2 h-4 w-4" />
            添加模型
          </Button>
        </div>

        <div className="space-y-3">
          {models.length === 0 ? (
            <div className="text-sm text-muted-foreground">尚未配置模型。</div>
          ) : null}

          {models.map((m) => {
            return (
              <div key={m.local_id} className="rounded-lg border bg-card p-3">
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold">
                      {(m.label || "").trim() || "未命名"}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => void onTestModel(m)}
                      aria-label="测试模型"
                      disabled={testingId !== null}
                      title="测试该模型配置（会产生少量调用）"
                    >
                      <Play className="mr-2 h-4 w-4" />
                      测试
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onDeleteModel(m.id)}
                      aria-label="删除 Model"
                      disabled={testingId === m.id}
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      删除
                    </Button>
                  </div>
                </div>

                <div className="mt-3 grid gap-3 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <div className="text-xs text-muted-foreground">ID</div>
                    <Input
                      value={m.id}
                      disabled={testingId === m.id}
                      onChange={(e) =>
                        setModels((prev) =>
                          prev.map((x) =>
                            x === m ? { ...x, id: e.target.value } : x,
                          ),
                        )
                      }
                      placeholder="codex"
                    />
                  </div>

                  <div className="grid gap-2">
                    <div className="text-xs text-muted-foreground">名称</div>
                    <Input
                      value={m.label}
                      disabled={testingId === m.id}
                      onChange={(e) =>
                        setModels((prev) =>
                          prev.map((x) =>
                            x === m ? { ...x, label: e.target.value } : x,
                          ),
                        )
                      }
                      placeholder="我的模型"
                    />
                  </div>

                  <div className="grid gap-2">
                    <div className="text-xs text-muted-foreground">SDK</div>
                    <Select
                      value={m.provider}
                      disabled={testingId === m.id}
                      onValueChange={(v) =>
                        setModels((prev) =>
                          prev.map((x) => {
                            if (x !== m) return x;
                            const nextSource = sourceOptions.some(
                              (o) => o.id === x.source_id,
                            )
                              ? x.source_id
                              : (sourceOptions[0]?.id ?? "");
                            return { ...x, provider: v, source_id: nextSource };
                          }),
                        )
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="选择 SDK" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="openai">OpenAI</SelectItem>
                        <SelectItem value="anthropic">Anthropic</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="grid gap-2">
                    <div className="text-xs text-muted-foreground">Source</div>
                    <Select
                      value={m.source_id}
                      disabled={testingId === m.id}
                      onValueChange={(v) =>
                        setModels((prev) =>
                          prev.map((x) =>
                            x === m ? { ...x, source_id: v } : x,
                          ),
                        )
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="选择 Source" />
                      </SelectTrigger>
                      <SelectContent>
                        {sourceOptions.length > 0 ? (
                          sourceOptions.map((s) => (
                            <SelectItem key={s.id} value={s.id}>
                              {formatSourceOption(s.id)}
                            </SelectItem>
                          ))
                        ) : (
                          <SelectItem value="">无可用 Source</SelectItem>
                        )}
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="grid gap-2 sm:col-span-2">
                    <div className="text-xs text-muted-foreground">模型名</div>
                    <Input
                      value={m.model}
                      disabled={testingId === m.id}
                      onChange={(e) =>
                        setModels((prev) =>
                          prev.map((x) =>
                            x === m ? { ...x, model: e.target.value } : x,
                          ),
                        )
                      }
                      placeholder="gpt-5-codex / claude-3-7-sonnet-latest"
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
        <Button
          variant="secondary"
          onClick={() => void load()}
          disabled={saving}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          重新加载
        </Button>
        <Button onClick={() => void onSave()} disabled={saving}>
          <Save className="mr-2 h-4 w-4" />
          {saving ? "保存中…" : "保存"}
        </Button>
      </div>
    </div>
  );
}
