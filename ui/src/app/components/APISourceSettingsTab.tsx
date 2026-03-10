import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Input, Skeleton } from "@heroui/react";
import { Plus, RefreshCw, Save, Trash2 } from "lucide-react";

import {
  fetchAPISourceSettings,
  putAPISourceSettings,
  type APISource,
  type PutAPISourceSettingsRequest,
} from "@/lib/daemon";
import { toast } from "@/lib/toast";
import { useDaemonStore } from "@/stores/daemonStore";

import {
  SETTINGS_INPUT_CLASSNAMES,
  SETTINGS_PANEL_BUTTON_CLASS,
  SettingsTabLayout,
} from "./settingsUi";

type SourceDraft = {
  local_id: string;
  id: string;
  label: string;
  base_url: string;
  has_key: boolean;
  masked_key: string;
  api_key: string;
};

type SourceFieldRowProps = {
  label: string;
  children: ReactNode;
  description?: string;
};

function SourceFieldRow(props: SourceFieldRowProps) {
  return (
    <div className="grid grid-cols-[55px_minmax(0,1fr)] items-center gap-3">
      <div className="text-xs font-medium text-muted-foreground">{props.label}</div>
      <div className="min-w-0">
        {props.children}
        {props.description ? (
          <div className="mt-1 text-xs text-muted-foreground">{props.description}</div>
        ) : null}
      </div>
    </div>
  );
}

function newLocalID(): string {
  return `${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function normalizeBaseUrl(raw: string): string {
  const value = raw.trim();
  if (!value) return "";
  return value.endsWith("/") ? value.slice(0, -1) : value;
}

function normalizeAuthMode(value: string): string {
  const normalized = value.trim().toLowerCase();
  if (normalized === "browser" || normalized === "api_key") return normalized;
  return "";
}

function isManagedSource(source: APISource): boolean {
  return Boolean(normalizeAuthMode(source.auth_mode ?? ""));
}

function toDraft(source: APISource): SourceDraft {
  return {
    local_id: newLocalID(),
    id: source.id ?? "",
    label: source.label ?? "",
    base_url: normalizeBaseUrl(source.base_url ?? ""),
    has_key: Boolean(source.has_key),
    masked_key: source.masked_key ?? "",
    api_key: "",
  };
}

function splitSources(sources: APISource[]): {
  visibleSources: APISource[];
  hiddenManagedSources: APISource[];
} {
  const visibleSources: APISource[] = [];
  const hiddenManagedSources: APISource[] = [];

  for (const source of sources) {
    if (isManagedSource(source)) {
      hiddenManagedSources.push(source);
      continue;
    }
    visibleSources.push(source);
  }

  return { visibleSources, hiddenManagedSources };
}

function buildRequest(
  drafts: SourceDraft[],
  hiddenManagedSources: APISource[],
): PutAPISourceSettingsRequest {
  return {
    sources: [
      ...drafts.map((item) => ({
        id: item.id.trim(),
        label: item.label.trim(),
        base_url: normalizeBaseUrl(item.base_url) || undefined,
        api_key: item.api_key.trim() || undefined,
      })),
      ...hiddenManagedSources.map((source) => ({
        id: source.id.trim(),
        label: source.label.trim(),
        base_url: normalizeBaseUrl(source.base_url ?? "") || undefined,
        auth_mode: normalizeAuthMode(source.auth_mode ?? "") || undefined,
      })),
    ],
  };
}

export function APISourceSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<SourceDraft[]>([]);
  const [hiddenManagedSources, setHiddenManagedSources] = useState<APISource[]>([]);

  const applyLoadedSources = useCallback((sources: APISource[]) => {
    const { visibleSources, hiddenManagedSources: managedSources } = splitSources(
      sources ?? [],
    );
    setDrafts(visibleSources.map(toDraft));
    setHiddenManagedSources(managedSources);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetchAPISourceSettings(daemonUrl);
      applyLoadedSources(res.sources ?? []);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      setDrafts([]);
      setHiddenManagedSources([]);
    } finally {
      setLoading(false);
    }
  }, [applyLoadedSources, daemonUrl]);

  useEffect(() => {
    void load();
  }, [load]);

  const canSave = useMemo(
    () => drafts.every((item) => item.label.trim()),
    [drafts],
  );

  const addSource = () => {
    setDrafts((prev) => [
      ...prev,
      {
        local_id: newLocalID(),
        id: "",
        label: "",
        base_url: "",
        has_key: false,
        masked_key: "",
        api_key: "",
      },
    ]);
  };

  const onSave = async () => {
    if (!canSave) {
      toast({ variant: "destructive", title: "请先补齐来源信息" });
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = await putAPISourceSettings(
        daemonUrl,
        buildRequest(drafts, hiddenManagedSources),
      );
      applyLoadedSources(saved.sources ?? []);
      toast({ title: "API 来源已保存" });
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

  return (
    <SettingsTabLayout
      footer={
        <>
          <div className="flex gap-2">
            <Button
              radius="full"
              size="sm"
              className={SETTINGS_PANEL_BUTTON_CLASS}
              variant="flat"
              startContent={<Plus className="h-4 w-4" />}
              onPress={addSource}
            >
              添加来源
            </Button>
            <Button
              radius="full"
              size="sm"
              className={SETTINGS_PANEL_BUTTON_CLASS}
              variant="flat"
              startContent={<RefreshCw className="h-4 w-4" />}
              onPress={() => void load()}
            >
              重新加载
            </Button>
          </div>
          <Button
            radius="full"
            size="sm"
            className={SETTINGS_PANEL_BUTTON_CLASS}
            color="primary"
            startContent={<Save className="h-4 w-4" />}
            isLoading={saving}
            onPress={() => void onSave()}
          >
            保存
          </Button>
        </>
      }
    >
      {error ? (
        <Alert color="danger" title="加载或保存失败" description={error} />
      ) : null}

      {loading ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-72 rounded-xl" />
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {drafts.map((item) => {
            const title = item.label.trim() || "未命名来源";
            const keyPlaceholder = item.has_key
              ? `已保存：${item.masked_key || "****"}`
              : "输入 API Key";

            return (
              <section
                key={item.local_id}
                className="space-y-4 rounded-xl border bg-background/40 p-4"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0 truncate text-sm font-medium">{title}</div>
                  <Button
                    size="sm"
                    radius="full"
                    className={SETTINGS_PANEL_BUTTON_CLASS}
                    variant="light"
                    color="danger"
                    startContent={<Trash2 className="h-4 w-4" />}
                    onPress={() =>
                      setDrafts((prev) =>
                        prev.filter((draft) => draft.local_id !== item.local_id),
                      )
                    }
                  >
                    删除
                  </Button>
                </div>

                <div className="space-y-3">
                  <SourceFieldRow label="名称">
                    <Input
                      radius="full"
                      size="sm"
                      classNames={SETTINGS_INPUT_CLASSNAMES}
                      aria-label="显示名称"
                      value={item.label}
                      onValueChange={(value) =>
                        setDrafts((prev) =>
                          prev.map((draft) =>
                            draft.local_id === item.local_id
                              ? { ...draft, label: value }
                              : draft,
                          ),
                        )
                      }
                      placeholder="例如 主网关"
                    />
                  </SourceFieldRow>

                  <SourceFieldRow label="地址">
                    <Input
                      radius="full"
                      size="sm"
                      classNames={SETTINGS_INPUT_CLASSNAMES}
                      aria-label="Base URL"
                      value={item.base_url}
                      onValueChange={(value) =>
                        setDrafts((prev) =>
                          prev.map((draft) =>
                            draft.local_id === item.local_id
                              ? { ...draft, base_url: value }
                              : draft,
                          ),
                        )
                      }
                      placeholder="例如 https://api.example.com/v1"
                    />
                  </SourceFieldRow>

                  <SourceFieldRow label="密钥">
                    <Input
                      radius="full"
                      size="sm"
                      classNames={SETTINGS_INPUT_CLASSNAMES}
                      aria-label="API Key"
                      type="password"
                      value={item.api_key}
                      onValueChange={(value) =>
                        setDrafts((prev) =>
                          prev.map((draft) =>
                            draft.local_id === item.local_id
                              ? { ...draft, api_key: value }
                              : draft,
                          ),
                        )
                      }
                      placeholder={keyPlaceholder}
                    />
                  </SourceFieldRow>
                </div>
              </section>
            );
          })}
        </div>
      )}
    </SettingsTabLayout>
  );
}
