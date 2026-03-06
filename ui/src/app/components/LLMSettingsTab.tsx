import { useCallback, useEffect, useState } from "react";
import { Play, Plus, RefreshCw, Save, Trash2 } from "lucide-react";

import {
  Alert,
  Button,
  Chip,
  Input,
  Select,
  SelectItem,
  Skeleton,
} from "@heroui/react";

import { toast } from "@/lib/toast";
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

type SourceModelDraft = {
  local_id: string;
  value: string;
};

type SourceDraft = {
  local_id: string;
  id: string;
  label: string;
  label_touched: boolean;
  provider: string;
  base_url: string;
  has_key: boolean;
  masked_key: string;
  key_input: string;
  key_touched: boolean;
  models: SourceModelDraft[];
};

function normalizeBaseUrl(raw: string): string {
  const url = (raw ?? "").trim();
  if (!url) return "";
  return url.endsWith("/") ? url.slice(0, -1) : url;
}

function normalizeProvider(raw: string): string {
  return (raw ?? "").trim().toLowerCase();
}

function normalizeModelIdentifier(raw: string): string {
  return (raw ?? "").trim().toLowerCase();
}

function formatProviderText(provider: string): string {
  if (provider === "anthropic") return "Anthropic";
  return "OpenAI";
}

function newLocalID(): string {
  return `${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function displayValueForModel(model: LLMModelProfile): string {
  const label = (model.label ?? "").trim();
  const normalizedLabel = normalizeModelIdentifier(label);
  const modelName = (model.model ?? "").trim();
  const normalizedModel = normalizeModelIdentifier(modelName);
  if (label && normalizedLabel === normalizedModel) return label;
  if (modelName) return modelName;
  return label;
}

function toDraftSources(
  sourceList: LLMSource[],
  modelList: LLMModelProfile[],
): SourceDraft[] {
  const modelsBySourceID = new Map<string, SourceModelDraft[]>();
  const providerBySourceID = new Map<string, string>();

  for (const model of modelList) {
    const sourceID = (model.source_id ?? "").trim();
    if (!sourceID) continue;
    const next = modelsBySourceID.get(sourceID) ?? [];
    next.push({
      local_id: newLocalID(),
      value: displayValueForModel(model),
    });
    modelsBySourceID.set(sourceID, next);
    if (!providerBySourceID.has(sourceID)) {
      providerBySourceID.set(sourceID, normalizeProvider(model.provider));
    }
  }

  return sourceList.map((source) => {
    const id = (source.id ?? "").trim();
    return {
      local_id: newLocalID(),
      id,
      label: (source.label ?? "").trim(),
      label_touched: false,
      provider:
        normalizeProvider(source.provider) ||
        providerBySourceID.get(id) ||
        "openai",
      base_url: normalizeBaseUrl(String(source.base_url ?? "")),
      has_key: Boolean(source.has_key),
      masked_key: String(source.masked_key ?? ""),
      key_input: "",
      key_touched: false,
      models: modelsBySourceID.get(id) ?? [],
    };
  });
}

function buildPutRequest(sources: SourceDraft[]): PutLLMSettingsRequest {
  const models: PutLLMSettingsRequest["models"] = [];

  for (const source of sources) {
    const provider = normalizeProvider(source.provider);
    const sourceID = source.id.trim();
    for (const model of source.models) {
      const display = (model.value ?? "").trim();
      const normalized = normalizeModelIdentifier(display);
      models.push({
        id: normalized,
        label: display || normalized,
        provider,
        model: normalized,
        source_id: sourceID,
      });
    }
  }

  return {
    sources: sources.map((source) => ({
      id: source.id.trim(),
      label: source.label.trim(),
      provider: normalizeProvider(source.provider),
      base_url: normalizeBaseUrl(source.base_url),
      ...(source.key_touched ? { api_key: source.key_input.trim() } : {}),
    })),
    models,
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

function selectionToString(keys: unknown): string {
  if (keys === "all") return "";
  if (keys instanceof Set) {
    const first = keys.values().next().value;
    if (typeof first === "string") return first;
    if (typeof first === "number") return String(first);
  }
  return "";
}

function firstValidationError(sources: SourceDraft[]): string | null {
  for (const source of sources) {
    if (!source.id.trim()) return "请先填写 API 源 ID。";
    if (!normalizeProvider(source.provider)) {
      return `请先为 Source「${source.label || source.id || "未命名"}」选择 SDK。`;
    }
    for (const model of source.models) {
      if (!normalizeModelIdentifier(model.value)) {
        return `Source「${source.label || source.id || "未命名"}」中存在空的模型 ID。`;
      }
    }
  }

  const used = new Set<string>();
  for (const source of sources) {
    for (const model of source.models) {
      const id = normalizeModelIdentifier(model.value);
      if (!id) continue;
      if (used.has(id)) {
        return `模型 ID「${id}」重复，请调整后再保存。`;
      }
      used.add(id);
    }
  }

  return null;
}

/**
 * 功能：管理设置页中的 LLM Source/模型配置，并将模型编辑收敛到 Source 卡片内。
 * 参数/返回：无显式入参；返回用于渲染设置页“模型”标签内容的 React 节点。
 * 失败场景：daemon 不可达、保存失败或测试失败时展示错误提示并保持草稿。
 * 副作用：读取/保存 daemon 配置，触发专家列表刷新，并可能发起真实 SDK 测试调用。
 */
export function LLMSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const setExperts = useDaemonStore((s) => s.setExperts);
  const setExpertsError = useDaemonStore((s) => s.setExpertsError);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [sources, setSources] = useState<SourceDraft[]>([]);
  const [testingId, setTestingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const res = await fetchLLMSettings(daemonUrl);
      setSources(toDraftSources(res.sources ?? [], res.models ?? []));
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

  const onAddSource = () => {
    const used = new Set(sources.map((source) => source.id));
    const id = nextID("source", used);
    setSources((prev) => [
      ...prev,
      {
        local_id: newLocalID(),
        id,
        label: id,
        label_touched: false,
        provider: "openai",
        base_url: "",
        has_key: false,
        masked_key: "",
        key_input: "",
        key_touched: true,
        models: [],
      },
    ]);
  };

  const onDeleteSource = (sourceLocalID: string) => {
    const source = sources.find((item) => item.local_id === sourceLocalID);
    if (!source) return;
    if (source.models.length > 0) {
      toast({
        variant: "destructive",
        title: "无法删除 API 源",
        description: `请先移除该 Source 下的 ${source.models.length} 个模型。`,
      });
      return;
    }
    setSources((prev) => prev.filter((item) => item.local_id !== sourceLocalID));
  };

  const onAddModel = (sourceLocalID: string) => {
    setSources((prev) =>
      prev.map((source) =>
        source.local_id === sourceLocalID
          ? {
              ...source,
              models: [...source.models, { local_id: newLocalID(), value: "" }],
            }
          : source,
      ),
    );
  };

  const onDeleteModel = (sourceLocalID: string, modelLocalID: string) => {
    setSources((prev) =>
      prev.map((source) =>
        source.local_id === sourceLocalID
          ? {
              ...source,
              models: source.models.filter((model) => model.local_id !== modelLocalID),
            }
          : source,
      ),
    );
  };

  const onSave = async () => {
    const validationError = firstValidationError(sources);
    if (validationError) {
      toast({
        variant: "destructive",
        title: "配置不完整",
        description: validationError,
      });
      return;
    }

    setSaving(true);
    setError(null);
    try {
      const req = buildPutRequest(sources);
      const saved: LLMSettings = await putLLMSettings(daemonUrl, req);
      setSources(toDraftSources(saved.sources ?? [], saved.models ?? []));

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

  const onTestModel = async (source: SourceDraft, model: SourceModelDraft) => {
    if (testingId) return;

    const provider = normalizeProvider(source.provider);
    const modelID = normalizeModelIdentifier(model.value);
    if (!provider || !modelID) {
      toast({
        variant: "destructive",
        title: "配置不完整",
        description: "请先填写 Source 的 SDK 和模型 ID。",
      });
      return;
    }

    const apiKey = (source.key_input ?? "").trim();
    if (!apiKey && !source.has_key) {
      toast({
        variant: "destructive",
        title: "缺少 API Key",
        description: "请先为该 Source 填写 API Key（或先保存已有 Key）。",
      });
      return;
    }

    setTestingId(model.local_id);
    try {
      const res = await postLLMTest(daemonUrl, {
        provider,
        model: modelID,
        source_id: source.id.trim(),
        base_url: normalizeBaseUrl(source.base_url),
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
        <Skeleton className="h-10 w-full rounded-md" />
        <Skeleton className="h-24 w-full rounded-md" />
        <Skeleton className="h-24 w-full rounded-md" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {error ? (
        <Alert color="danger" title="加载/保存失败" description={error} />
      ) : null}

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium">API 源</div>
          <Button
            color="secondary"
            variant="flat"
            size="sm"
            onPress={onAddSource}
            startContent={<Plus className="h-4 w-4" />}
          >
            添加来源
          </Button>
        </div>

        <div className="space-y-3">
          {sources.length === 0 ? (
            <div className="text-sm text-muted-foreground">尚未配置 API 源。</div>
          ) : null}

          {sources.map((source) => (
            <div key={source.local_id} className="rounded-lg border bg-card p-3">
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="truncate text-sm font-semibold">
                    {(source.label || "").trim() || "未命名"}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2">
                    <Chip variant="bordered" size="sm">
                      SDK：{formatProviderText(normalizeProvider(source.provider))}
                    </Chip>
                    {source.has_key ? (
                      <Chip variant="bordered" size="sm">
                        Key：{source.masked_key || "已设置"}
                      </Chip>
                    ) : (
                      <Chip variant="bordered" size="sm">
                        Key：未设置
                      </Chip>
                    )}
                  </div>
                </div>
                <Button
                  variant="light"
                  size="sm"
                  isIconOnly
                  onPress={() => onDeleteSource(source.local_id)}
                  aria-label="删除 Source"
                >
                  <Trash2 className="h-4 w-4" aria-hidden="true" focusable="false" />
                </Button>
              </div>

              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">ID</div>
                  <Input
                    value={source.id}
                    onValueChange={(value) =>
                      setSources((prev) =>
                        prev.map((item) => {
                          if (item.local_id !== source.local_id) return item;
                          const currentID = item.id.trim();
                          const shouldSyncLabel =
                            !item.label_touched || item.label.trim() === currentID;
                          return {
                            ...item,
                            id: value,
                            label: shouldSyncLabel ? value : item.label,
                          };
                        }),
                      )
                    }
                    placeholder="source"
                  />
                </div>

                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">名称</div>
                  <Input
                    value={source.label}
                    onValueChange={(label) =>
                      setSources((prev) =>
                        prev.map((item) =>
                          item.local_id === source.local_id
                            ? { ...item, label, label_touched: true }
                            : item,
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
                    value={source.base_url}
                    onValueChange={(base_url) =>
                      setSources((prev) =>
                        prev.map((item) =>
                          item.local_id === source.local_id
                            ? { ...item, base_url }
                            : item,
                        ),
                      )
                    }
                    placeholder="https://api.example.com"
                  />
                </div>

                <div className="grid gap-2">
                  <div className="text-xs text-muted-foreground">SDK</div>
                  <Select
                    aria-label="SDK"
                    placeholder="选择 SDK"
                    selectionMode="single"
                    disallowEmptySelection
                    selectedKeys={
                      source.provider ? new Set([source.provider]) : new Set([])
                    }
                    onSelectionChange={(keys) =>
                      setSources((prev) =>
                        prev.map((item) =>
                          item.local_id === source.local_id
                            ? {
                                ...item,
                                provider: selectionToString(keys) || item.provider,
                              }
                            : item,
                        ),
                      )
                    }
                  >
                    <SelectItem key="openai">OpenAI</SelectItem>
                    <SelectItem key="anthropic">Anthropic</SelectItem>
                  </Select>
                </div>

                <div className="grid gap-2 sm:col-span-2">
                  <div className="text-xs text-muted-foreground">
                    API Key（留空表示不修改；空字符串表示清空）
                  </div>
                  <Input
                    type="password"
                    value={source.key_input}
                    onValueChange={(key_input) =>
                      setSources((prev) =>
                        prev.map((item) =>
                          item.local_id === source.local_id
                            ? { ...item, key_input, key_touched: true }
                            : item,
                        ),
                      )
                    }
                    placeholder={source.has_key ? "留空不修改" : "sk-..."}
                    autoComplete="off"
                  />
                </div>
              </div>

              <div className="mt-4 rounded-md border border-dashed bg-muted/20 p-3">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <div className="text-sm font-medium">模型</div>
                    <div className="text-xs text-muted-foreground">
                      这里直接录入可用模型 ID；显示可保留大写，保存和测试时会自动转成小写。
                    </div>
                  </div>
                  <Button
                    color="secondary"
                    variant="flat"
                    size="sm"
                    onPress={() => onAddModel(source.local_id)}
                    startContent={<Plus className="h-4 w-4" />}
                  >
                    添加模型
                  </Button>
                </div>

                <div className="mt-3 space-y-3">
                  {source.models.length === 0 ? (
                    <div className="text-sm text-muted-foreground">
                      尚未为该 Source 配置模型。
                    </div>
                  ) : null}

                  {source.models.map((model) => {
                    const normalizedModel = normalizeModelIdentifier(model.value);
                    const isTesting = testingId === model.local_id;
                    return (
                      <div key={model.local_id} className="rounded-md border bg-card p-3">
                        <div className="flex items-center justify-between gap-2">
                          <div className="min-w-0">
                            <div className="truncate text-sm font-semibold">
                              {(model.value || "").trim() || "未命名模型"}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              请求使用：{normalizedModel || "保存/测试时自动转小写"}
                            </div>
                          </div>
                          <div className="flex items-center gap-1">
                            <Button
                              variant="light"
                              size="sm"
                              onPress={() => void onTestModel(source, model)}
                              aria-label="测试模型"
                              isDisabled={testingId !== null}
                              title="测试该模型配置（会产生少量调用）"
                            >
                              <Play className="mr-2 h-4 w-4" aria-hidden="true" focusable="false" />
                              测试
                            </Button>
                            <Button
                              variant="light"
                              size="sm"
                              onPress={() => onDeleteModel(source.local_id, model.local_id)}
                              aria-label="删除模型"
                              isDisabled={isTesting}
                            >
                              <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" focusable="false" />
                              删除
                            </Button>
                          </div>
                        </div>

                        <div className="mt-3 grid gap-2">
                          <div className="text-xs text-muted-foreground">模型 ID</div>
                          <Input
                            value={model.value}
                            isDisabled={isTesting}
                            onValueChange={(value) =>
                              setSources((prev) =>
                                prev.map((item) =>
                                  item.local_id === source.local_id
                                    ? {
                                        ...item,
                                        models: item.models.map((entry) =>
                                          entry.local_id === model.local_id
                                            ? { ...entry, value }
                                            : entry,
                                        ),
                                      }
                                    : item,
                                ),
                              )
                            }
                            placeholder={
                              normalizeProvider(source.provider) === "anthropic"
                                ? "CLAUDE-3-7-SONNET-LATEST"
                                : "GPT-5-CODEX"
                            }
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
        <Button
          color="secondary"
          variant="flat"
          onPress={() => void load()}
          isDisabled={saving}
          startContent={
            <RefreshCw className="h-4 w-4" aria-hidden="true" focusable="false" />
          }
        >
          重新加载
        </Button>
        <Button
          color="primary"
          onPress={() => void onSave()}
          isDisabled={saving}
          startContent={<Save className="h-4 w-4" aria-hidden="true" focusable="false" />}
        >
          {saving ? "保存中…" : "保存"}
        </Button>
      </div>
    </div>
  );
}
