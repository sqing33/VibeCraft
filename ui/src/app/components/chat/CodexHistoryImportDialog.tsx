import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
} from "@heroui/react";
import { Download, RefreshCcw, Search } from "lucide-react";

import {
  fetchCodexHistoryThreads,
  importCodexHistoryThreads,
  type CodexHistoryThread,
} from "@/lib/daemon";
import { formatRelativeTime } from "@/lib/time";

type CodexHistoryImportDialogProps = {
  daemonUrl: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImported?: (sessionIds: string[]) => void | Promise<void>;
};

export function CodexHistoryImportDialog({
  daemonUrl,
  open,
  onOpenChange,
  onImported,
}: CodexHistoryImportDialogProps) {
  const [threads, setThreads] = useState<CodexHistoryThread[]>([]);
  const [selectedThreadIDs, setSelectedThreadIDs] = useState<Set<string>>(
    new Set(),
  );
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadThreads = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const nextThreads = await fetchCodexHistoryThreads(daemonUrl);
      setThreads(nextThreads);
      setSelectedThreadIDs((prev) => {
        const next = new Set<string>();
        for (const threadID of prev) {
          if (nextThreads.some((item) => item.thread_id === threadID)) {
            next.add(threadID);
          }
        }
        return next;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [daemonUrl]);

  useEffect(() => {
    if (!open) return;
    void loadThreads();
  }, [open, loadThreads]);

  const filteredThreads = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return threads;
    return threads.filter((thread) => {
      const title = thread.display_title.toLowerCase();
      const workspace = thread.workspace_path.toLowerCase();
      return title.includes(keyword) || workspace.includes(keyword);
    });
  }, [query, threads]);

  const selectedCount = selectedThreadIDs.size;

  const selectableThreadIDs = useMemo(
    () =>
      filteredThreads
        .filter((thread) => !thread.already_imported)
        .map((thread) => thread.thread_id),
    [filteredThreads],
  );

  const toggleSelected = (threadID: string) => {
    setSelectedThreadIDs((prev) => {
      const next = new Set(prev);
      if (next.has(threadID)) next.delete(threadID);
      else next.add(threadID);
      return next;
    });
  };

  const selectVisible = () => {
    setSelectedThreadIDs((prev) => {
      const next = new Set(prev);
      for (const threadID of selectableThreadIDs) next.add(threadID);
      return next;
    });
  };

  const clearSelected = () => {
    setSelectedThreadIDs(new Set());
  };

  const onConfirmImport = async () => {
    const threadIDs = Array.from(selectedThreadIDs);
    if (threadIDs.length === 0) return;
    setImporting(true);
    setError(null);
    try {
      const result = await importCodexHistoryThreads(daemonUrl, {
        thread_ids: threadIDs,
      });
      const sessionIDs = result.results
        .filter((item) => item.session_id)
        .map((item) => item.session_id as string);
      await onImported?.(sessionIDs);
      onOpenChange(false);
      setSelectedThreadIDs(new Set());
      setQuery("");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setImporting(false);
    }
  };

  return (
    <Modal
      isOpen={open}
      onOpenChange={onOpenChange}
      size="4xl"
      scrollBehavior="inside"
      classNames={{ base: "max-h-[80vh]" }}
    >
      <ModalContent>
        {() => (
          <>
            <ModalHeader className="flex items-center gap-2">
              <Download className="h-5 w-5" />
              导入 Codex 历史
            </ModalHeader>
            <ModalBody className="space-y-4">
              <div className="flex items-center gap-2">
                <Input
                  placeholder="按标题或工作目录搜索"
                  value={query}
                  onValueChange={setQuery}
                  startContent={<Search className="h-4 w-4 text-default-400" />}
                />
                <Button
                  variant="flat"
                  isIconOnly
                  aria-label="刷新 Codex 历史"
                  onPress={() => void loadThreads()}
                  isDisabled={loading || importing}
                >
                  <RefreshCcw className="h-4 w-4" />
                </Button>
              </div>

              <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
                <span>
                  共 {threads.length} 条，当前筛选 {filteredThreads.length} 条，已选{" "}
                  {selectedCount} 条
                </span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    className="rounded-md border px-2 py-1 transition hover:bg-default-100"
                    onClick={selectVisible}
                    disabled={selectableThreadIDs.length === 0}
                  >
                    选中当前可导入项
                  </button>
                  <button
                    type="button"
                    className="rounded-md border px-2 py-1 transition hover:bg-default-100"
                    onClick={clearSelected}
                    disabled={selectedCount === 0}
                  >
                    清空
                  </button>
                </div>
              </div>

              {error ? (
                <Alert color="danger" title="读取 Codex 历史失败" description={error} />
              ) : null}

              <div className="max-h-[52vh] space-y-2 overflow-y-auto pr-1">
                {loading ? (
                  <div className="rounded-2xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
                    正在读取 `~/.codex` 历史…
                  </div>
                ) : filteredThreads.length === 0 ? (
                  <div className="rounded-2xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
                    没有匹配的历史记录
                  </div>
                ) : (
                  filteredThreads.map((thread) => {
                    const selected = selectedThreadIDs.has(thread.thread_id);
                    return (
                      <label
                        key={thread.thread_id}
                        className={`flex cursor-pointer items-start gap-3 rounded-2xl border px-4 py-3 transition ${
                          selected
                            ? "border-primary/50 bg-primary/5"
                            : "border-default-200/70 bg-background/70 hover:border-default-300"
                        } ${thread.already_imported ? "opacity-70" : ""}`}
                      >
                        <input
                          type="checkbox"
                          className="mt-1 h-4 w-4"
                          checked={selected}
                          disabled={thread.already_imported}
                          onChange={() => toggleSelected(thread.thread_id)}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">
                                {thread.display_title}
                              </div>
                              <div className="mt-1 truncate text-xs text-muted-foreground">
                                {thread.workspace_path || "-"}
                              </div>
                            </div>
                            <div className="shrink-0 text-right text-[11px] text-muted-foreground">
                              <div>{formatRelativeTime(thread.updated_at)}</div>
                              <div className="mt-1">
                                {thread.already_imported ? "已导入" : "可导入"}
                              </div>
                            </div>
                          </div>
                        </div>
                      </label>
                    );
                  })
                )}
              </div>
            </ModalBody>
            <ModalFooter>
              <Button variant="light" onPress={() => onOpenChange(false)}>
                取消
              </Button>
              <Button
                color="primary"
                onPress={() => void onConfirmImport()}
                isLoading={importing}
                isDisabled={selectedCount === 0 || importing}
              >
                导入所选 {selectedCount > 0 ? `(${selectedCount})` : ""}
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
