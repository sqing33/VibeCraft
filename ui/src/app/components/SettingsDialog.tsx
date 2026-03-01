import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Settings2 } from "lucide-react";

import { toast } from "@/components/ui/use-toast";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { copyToClipboard } from "@/lib/clipboard";
import { daemonUrlFromEnv } from "@/lib/daemon";
import { type HealthState, useDaemonStore } from "@/stores/daemonStore";

import { LLMSettingsTab } from "./LLMSettingsTab";

function normalizeBaseUrl(raw: string): string {
  const url = (raw ?? "").trim();
  if (!url) return "";
  return url.endsWith("/") ? url.slice(0, -1) : url;
}

function healthBadge(health: HealthState) {
  if (health.status === "checking") {
    return <Badge variant="secondary">健康状态：检查中</Badge>;
  }
  if (health.status === "ok") {
    return (
      <Badge className="bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-200">
        健康状态：正常
      </Badge>
    );
  }
  return (
    <Badge className="bg-red-500/15 text-red-700 hover:bg-red-500/15 dark:text-red-200">
      健康状态：异常
    </Badge>
  );
}

function wsText(state: string): string {
  if (state === "connected") return "已连接";
  if (state === "connecting") return "连接中";
  return "未连接";
}

export function SettingsDialog() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const health = useDaemonStore((s) => s.health);
  const wsState = useDaemonStore((s) => s.wsState);
  const info = useDaemonStore((s) => s.info);
  const infoError = useDaemonStore((s) => s.infoError);
  const experts = useDaemonStore((s) => s.experts);
  const expertsError = useDaemonStore((s) => s.expertsError);
  const setDaemonUrl = useDaemonStore((s) => s.setDaemonUrl);
  const resetDaemonUrl = useDaemonStore((s) => s.resetDaemonUrl);

  const [open, setOpen] = useState(false);
  const [daemonUrlInput, setDaemonUrlInput] = useState(daemonUrl);
  const [daemonUrlError, setDaemonUrlError] = useState<string | null>(null);

  useEffect(() => {
    setDaemonUrlInput(daemonUrl);
  }, [daemonUrl]);

  const isUsingEnvDefault = useMemo(() => {
    return normalizeBaseUrl(daemonUrl) === normalizeBaseUrl(daemonUrlFromEnv());
  }, [daemonUrl]);

  const onApplyDaemonUrl = () => {
    setDaemonUrlError(null);
    const normalized = normalizeBaseUrl(daemonUrlInput);
    if (!normalized) {
      setDaemonUrlError("守护进程地址不能为空。");
      return;
    }
    try {
      const parsed = new URL(normalized);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        setDaemonUrlError("守护进程地址必须以 http:// 或 https:// 开头。");
        return;
      }
    } catch {
      setDaemonUrlError("守护进程地址格式不正确。");
      return;
    }

    setDaemonUrl(normalized);
    toast({
      title: "守护进程地址已更新",
      description: normalized,
    });
  };

  const onResetDaemonUrl = () => {
    resetDaemonUrl();
    setDaemonUrlError(null);
    toast({
      title: "守护进程地址已重置",
      description: daemonUrlFromEnv(),
    });
  };

  const onCopy = async (label: string, text: string) => {
    try {
      await copyToClipboard(text);
      toast({ title: "已复制", description: label });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      toast({
        variant: "destructive",
        title: "复制失败",
        description: message,
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="设置">
          <Settings2 />
        </Button>
      </DialogTrigger>
      <DialogContent className="flex max-h-[85vh] min-h-0 max-w-2xl flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>系统设置</DialogTitle>
        </DialogHeader>

        <Tabs
          defaultValue="diagnostics"
          className="flex min-h-0 flex-1 flex-col"
        >
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="diagnostics">连接与诊断</TabsTrigger>
            <TabsTrigger value="llm">模型</TabsTrigger>
          </TabsList>

          <div className="min-h-0 flex-1 overflow-y-auto pr-1">
            <TabsContent value="diagnostics">
              <div className="space-y-6">
                <section className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    {healthBadge(health)}
                    <Badge variant="secondary">连接：{wsText(wsState)}</Badge>
                    {isUsingEnvDefault ? (
                      <Badge variant="outline">使用环境变量默认值</Badge>
                    ) : (
                      <Badge variant="outline">运行时覆盖</Badge>
                    )}
                  </div>

                  <div className="grid gap-2">
                    <div className="text-sm font-medium">守护进程地址</div>
                    <div className="flex flex-col gap-2 sm:flex-row">
                      <Input
                        value={daemonUrlInput}
                        onChange={(e) => setDaemonUrlInput(e.target.value)}
                        placeholder="http://127.0.0.1:7777"
                      />
                      <div className="flex gap-2">
                        <Button onClick={onApplyDaemonUrl}>
                          <Check className="mr-2 h-4 w-4" />
                          应用
                        </Button>
                        <Button variant="secondary" onClick={onResetDaemonUrl}>
                          重置
                        </Button>
                      </div>
                    </div>

                    {daemonUrlError ? (
                      <Alert variant="destructive">
                        <AlertTitle>守护进程地址无效</AlertTitle>
                        <AlertDescription>{daemonUrlError}</AlertDescription>
                      </Alert>
                    ) : null}

                    <div className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2">
                      <code className="truncate text-xs">{daemonUrl}</code>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => void onCopy("守护进程地址", daemonUrl)}
                        aria-label="复制守护进程地址"
                      >
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </section>

                <section className="space-y-3">
                  <div className="text-sm font-medium">版本信息</div>
                  {info ? (
                    <div className="rounded-md border bg-muted/30 p-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <code className="text-xs">{info.version.commit}</code>
                        {info.version.built_at ? (
                          <span className="text-xs text-muted-foreground">
                            {info.version.built_at}
                          </span>
                        ) : null}
                      </div>
                    </div>
                  ) : infoError ? (
                    <Alert variant="destructive">
                      <AlertTitle>加载信息失败</AlertTitle>
                      <AlertDescription>{infoError}</AlertDescription>
                    </Alert>
                  ) : (
                    <Skeleton className="h-10 w-full" />
                  )}
                </section>

                <section className="space-y-3">
                  <div className="text-sm font-medium">路径信息</div>
                  {info ? (
                    <div className="grid gap-2">
                      {[
                        { label: "配置", value: info.paths.config_path },
                        { label: "数据", value: info.paths.data_dir },
                        { label: "日志", value: info.paths.logs_dir },
                        { label: "SQLite", value: info.paths.state_db_path },
                      ].map((p) => (
                        <div
                          key={p.label}
                          className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2"
                        >
                          <div className="min-w-0">
                            <div className="text-xs text-muted-foreground">
                              {p.label}
                            </div>
                            <code className="block truncate text-xs">
                              {p.value}
                            </code>
                          </div>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => void onCopy(p.label, p.value)}
                            aria-label={`复制${p.label}路径`}
                          >
                            <Copy className="h-4 w-4" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  ) : infoError ? null : (
                    <div className="grid gap-2">
                      <Skeleton className="h-12 w-full" />
                      <Skeleton className="h-12 w-full" />
                      <Skeleton className="h-12 w-full" />
                      <Skeleton className="h-12 w-full" />
                    </div>
                  )}
                </section>

                <section className="space-y-3">
                  <div className="text-sm font-medium">专家列表</div>
                  {experts.length > 0 ? (
                    <div className="rounded-md border bg-muted/30 p-3 text-xs">
                      <div className="text-muted-foreground">
                        已配置 {experts.length} 个
                      </div>
                      <div className="mt-1 flex flex-wrap gap-1">
                        {experts.map((e) => (
                          <Badge key={e.id} variant="secondary">
                            {e.id}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  ) : expertsError ? (
                    <Alert variant="destructive">
                      <AlertTitle>加载专家列表失败</AlertTitle>
                      <AlertDescription>{expertsError}</AlertDescription>
                    </Alert>
                  ) : health.status === "error" ? (
                    <div className="text-xs text-muted-foreground">
                      无法使用专家列表（守护进程不可达）。
                    </div>
                  ) : (
                    <Skeleton className="h-16 w-full" />
                  )}
                </section>
              </div>
            </TabsContent>

            <TabsContent value="llm">
              <LLMSettingsTab />
            </TabsContent>
          </div>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
