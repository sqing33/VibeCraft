import { useEffect, useMemo, useState } from "react";
import { Check, Copy, Settings2 } from "lucide-react";
import {
  Alert,
  Button,
  Chip,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalHeader,
  Skeleton,
  Tab,
  Tabs,
} from "@heroui/react";

import { toast } from "@/lib/toast";
import { copyToClipboard } from "@/lib/clipboard";
import { daemonUrlFromEnv } from "@/lib/daemon";
import { type HealthState, useDaemonStore } from "@/stores/daemonStore";

import { BasicSettingsTab } from "./BasicSettingsTab";
import { CLIToolSettingsTab } from "./CLIToolSettingsTab";
import { LLMSettingsTab } from "./LLMSettingsTab";
import { ExpertSettingsTab } from "./ExpertSettingsTab";
import { MCPSettingsTab } from "./MCPSettingsTab";
import { SkillSettingsTab } from "./SkillSettingsTab";

function normalizeBaseUrl(raw: string): string {
  const url = (raw ?? "").trim();
  if (!url) return "";
  return url.endsWith("/") ? url.slice(0, -1) : url;
}

function healthBadge(health: HealthState) {
  if (health.status === "checking") {
    return (
      <Chip variant="flat" size="sm">
        健康状态：检查中
      </Chip>
    );
  }
  if (health.status === "ok") {
    return (
      <Chip color="success" variant="flat" size="sm">
        健康状态：正常
      </Chip>
    );
  }
  return (
    <Chip color="danger" variant="flat" size="sm">
      健康状态：异常
    </Chip>
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
    <>
      <Button
        variant="light"
        size="sm"
        isIconOnly
        aria-label="设置"
        onPress={() => setOpen(true)}
      >
        <Settings2 className="h-4 w-4" aria-hidden="true" focusable="false" />
      </Button>

      <Modal
        isOpen={open}
        onOpenChange={setOpen}
        size="5xl"
        scrollBehavior="inside"
        classNames={{
          base: "h-[80vh] max-h-[85vh] min-h-0",
        }}
      >
        <ModalContent className="h-full min-h-0 overflow-hidden">
          {() => (
            <>
              <ModalHeader>系统设置</ModalHeader>
              <ModalBody className="flex min-h-0 flex-1 overflow-hidden">
                <Tabs
                  defaultSelectedKey="basic"
                  aria-label="系统设置"
                  classNames={{
                    base: "w-full shrink-0",
                    panel: "h-full min-h-0 flex-1 overflow-y-auto pr-1",
                    tabList: "grid w-full shrink-0 grid-cols-7",
                  }}
                >
                  <Tab key="basic" title="基本设置">
                    <div className="flex min-h-full flex-col">
                      <BasicSettingsTab />
                    </div>
                  </Tab>
                  <Tab key="diagnostics" title="连接与诊断">
                    <div className="flex min-h-full flex-col space-y-6">
                      <section className="space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          {healthBadge(health)}
                          <Chip variant="flat" size="sm">
                            连接：{wsText(wsState)}
                          </Chip>
                          {isUsingEnvDefault ? (
                            <Chip variant="bordered" size="sm">
                              使用环境变量默认值
                            </Chip>
                          ) : (
                            <Chip variant="bordered" size="sm">
                              运行时覆盖
                            </Chip>
                          )}
                        </div>

                        <div className="grid gap-2">
                          <div className="text-sm font-medium">
                            守护进程地址
                          </div>
                          <div className="flex flex-col gap-2 sm:flex-row">
                            <Input
                              value={daemonUrlInput}
                              onValueChange={setDaemonUrlInput}
                              placeholder="http://127.0.0.1:7777"
                              className="flex-1"
                            />
                            <div className="flex gap-2">
                              <Button
                                color="primary"
                                onPress={onApplyDaemonUrl}
                                startContent={<Check className="h-4 w-4" />}
                              >
                                应用
                              </Button>
                              <Button
                                variant="flat"
                                onPress={onResetDaemonUrl}
                              >
                                重置
                              </Button>
                            </div>
                          </div>

                          {daemonUrlError ? (
                            <Alert
                              color="danger"
                              title="守护进程地址无效"
                              description={daemonUrlError}
                            />
                          ) : null}

                          <div className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2">
                            <code className="truncate text-xs">
                              {daemonUrl}
                            </code>
                            <Button
                              variant="light"
                              size="sm"
                              isIconOnly
                              onPress={() =>
                                void onCopy("守护进程地址", daemonUrl)
                              }
                              aria-label="复制守护进程地址"
                            >
                              <Copy
                                className="h-4 w-4"
                                aria-hidden="true"
                                focusable="false"
                              />
                            </Button>
                          </div>
                        </div>
                      </section>

                      <section className="space-y-3">
                        <div className="text-sm font-medium">版本信息</div>
                        {info ? (
                          <div className="rounded-md border bg-muted/30 p-3">
                            <div className="flex flex-wrap items-center gap-2">
                              <code className="text-xs">
                                {info.version.commit}
                              </code>
                              {info.version.built_at ? (
                                <span className="text-xs text-muted-foreground">
                                  {info.version.built_at}
                                </span>
                              ) : null}
                            </div>
                          </div>
                        ) : infoError ? (
                          <Alert
                            color="danger"
                            title="加载信息失败"
                            description={infoError}
                          />
                        ) : (
                          <Skeleton className="h-10 w-full rounded-md" />
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
                                  variant="light"
                                  size="sm"
                                  isIconOnly
                                  onPress={() => void onCopy(p.label, p.value)}
                                  aria-label={`复制${p.label}路径`}
                                >
                                  <Copy
                                    className="h-4 w-4"
                                    aria-hidden="true"
                                    focusable="false"
                                  />
                                </Button>
                              </div>
                            ))}
                          </div>
                        ) : infoError ? null : (
                          <div className="grid gap-2">
                            <Skeleton className="h-12 w-full rounded-md" />
                            <Skeleton className="h-12 w-full rounded-md" />
                            <Skeleton className="h-12 w-full rounded-md" />
                            <Skeleton className="h-12 w-full rounded-md" />
                          </div>
                        )}
                      </section>
                    </div>
                  </Tab>

                  <Tab key="cli-tools" title="CLI 工具">
                    <CLIToolSettingsTab />
                  </Tab>

                  <Tab key="mcp" title="MCP">
                    <MCPSettingsTab />
                  </Tab>

                  <Tab key="skills" title="技能">
                    <SkillSettingsTab />
                  </Tab>

                  <Tab key="llm" title="模型">
                    <LLMSettingsTab />
                  </Tab>

                  <Tab key="experts" title="专家">
                    <ExpertSettingsTab />
                  </Tab>
                </Tabs>
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>
    </>
  );
}
