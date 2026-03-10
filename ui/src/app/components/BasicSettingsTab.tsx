import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Select, SelectItem, Skeleton } from "@heroui/react";

import {
  fetchBasicSettings,
  fetchRuntimeModelSettings,
  putBasicSettings,
  type BasicSettings,
  type RuntimeModelSettings,
} from "@/lib/daemon";
import { flattenRuntimeModels } from "@/lib/runtimeModels";
import { toast } from "@/lib/toast";
import { useDaemonStore } from "@/stores/daemonStore";

import {
  SETTINGS_PANEL_BUTTON_CLASS,
  SETTINGS_SELECT_CLASSNAMES,
  SettingsTabLayout,
} from "./settingsUi";

type ThinkingTranslationDraft = {
  model_id: string;
};

function draftFromSettings(settings: BasicSettings): ThinkingTranslationDraft {
  return {
    model_id: settings.thinking_translation?.model_id?.trim() ?? "",
  };
}

function selectionToString(keys: "all" | Set<React.Key>): string {
  if (keys === "all") return "";
  return Array.from(keys)[0]?.toString() ?? "";
}

export function BasicSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [runtimeSettings, setRuntimeSettings] =
    useState<RuntimeModelSettings | null>(null);
  const [draft, setDraft] = useState<ThinkingTranslationDraft>({
    model_id: "",
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [basic, runtimeModels] = await Promise.all([
        fetchBasicSettings(daemonUrl),
        fetchRuntimeModelSettings(daemonUrl),
      ]);
      setDraft(draftFromSettings(basic));
      setRuntimeSettings(runtimeModels);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      setRuntimeSettings(null);
    } finally {
      setLoading(false);
    }
  }, [daemonUrl]);

  useEffect(() => {
    void load();
  }, [load]);

  const allModels = useMemo(
    () => flattenRuntimeModels(runtimeSettings),
    [runtimeSettings],
  );
  const translationModels = useMemo(
    () =>
      allModels.filter(
        (model) =>
          model.kind === "sdk" &&
          (model.provider === "openai" || model.provider === "anthropic"),
      ),
    [allModels],
  );
  const hasTranslationModels = translationModels.length > 0;

  const onSave = async () => {
    if (!draft.model_id.trim()) {
      toast({ variant: "destructive", title: "请先选择翻译模型" });
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = await putBasicSettings(daemonUrl, {
        thinking_translation: {
          model_id: draft.model_id.trim(),
        },
      });
      setDraft(draftFromSettings(saved));
      toast({ title: "基本设置已保存" });
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
          <Button
            radius="full"
            size="sm"
            className={SETTINGS_PANEL_BUTTON_CLASS}
            variant="flat"
            onPress={() => void load()}
          >
            重新加载
          </Button>
          <Button
            radius="full"
            size="sm"
            className={SETTINGS_PANEL_BUTTON_CLASS}
            color="primary"
            isLoading={saving}
            isDisabled={!hasTranslationModels}
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
        <div className="space-y-3 rounded-xl border bg-background/40 p-4">
          <Skeleton className="h-20 rounded-xl" />
          <Skeleton className="h-24 rounded-xl" />
        </div>
      ) : (
        <div className="space-y-4 rounded-xl border bg-background/40 p-4">
          {!hasTranslationModels ? (
            <Alert
              color="warning"
              title="请先配置 SDK 翻译模型"
              description="请先到“模型设置”页为 OpenAI SDK 或 Anthropic SDK 添加至少一个模型，然后再配置思考过程翻译。"
            />
          ) : null}

          <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
            <div className="min-w-0 flex-1 space-y-2">
              <div className="text-sm font-semibold">思考过程翻译</div>
              <div className="text-xs text-muted-foreground">
                选择一个 SDK
                模型用于将非中文思考过程自动翻译为简体中文。系统会在运行时自动判断当前思考过程是否需要翻译。
              </div>
            </div>

            <div className="w-full shrink-0 xl:w-[262px] xl:max-w-[262px]">
              <div className="grid grid-cols-[50px_minmax(0,1fr)] items-center gap-3 xl:grid-cols-[50px_200px]">
                <div className="text-xs font-medium text-muted-foreground">
                  翻译模型
                </div>
                <Select
                  radius="full"
                  size="sm"
                  classNames={SETTINGS_SELECT_CLASSNAMES}
                  aria-label="翻译模型"
                  placeholder={
                    hasTranslationModels
                      ? "请选择用于翻译的 SDK 模型"
                      : "请先到模型设置页配置 SDK 模型"
                  }
                  selectedKeys={
                    draft.model_id ? new Set([draft.model_id]) : new Set([])
                  }
                  selectionMode="single"
                  isDisabled={!hasTranslationModels}
                  onSelectionChange={(keys) =>
                    setDraft((prev) => ({
                      ...prev,
                      model_id: selectionToString(keys),
                    }))
                  }
                >
                  {translationModels.map((model) => (
                    <SelectItem
                      key={model.id}
                      textValue={model.label || model.id}
                    >
                      <div className="flex flex-col">
                        <span>{model.label || model.id}</span>
                        <span className="text-xs text-muted-foreground">
                          {model.runtime_label}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </Select>
              </div>
            </div>
          </div>
        </div>
      )}
    </SettingsTabLayout>
  );
}
