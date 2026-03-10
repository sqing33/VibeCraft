import {
  type DragEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Alert,
  Button,
  Chip,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Select,
  SelectItem,
  Switch,
} from "@heroui/react";
import { ArrowUp, ChevronUp, Eye, Plus, Trash2, X } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { goToChat } from "@/app/routes";
import { RuntimeIdentityMark } from "@/app/components/RuntimeIdentityMark";
import { OpenAIIcon } from "@/app/components/OpenAIIcon";
import { AnthropicIcon } from "@/app/components/AnthropicIcon";
import { OpenCodeIcon } from "@/app/components/OpenCodeIcon";
import { IFlowIcon } from "@/app/components/IFlowIcon";
import { WorkspacePortal } from "@/app/components/WorkspaceShell";
import { onWsEnvelope } from "@/lib/wsBus";
import {
  chatAttachmentContentUrl,
  cliToolPrimaryProtocolFamily,
  fetchCLIToolSettings,
  fetchMCPSettings,
  fetchRuntimeModelSettings,
  type ChatAttachment,
  type ChatSession,
  type CLITool,
  type MCPServerSetting,
  type MCPSettings,
  type RuntimeModelProfile,
  type RuntimeModelSettings,
} from "@/lib/daemon";
import {
  AttachmentPreviewModal,
  type AttachmentPreviewState,
} from "@/app/components/AttachmentPreviewModal";
import { ChatTurnFeed as ChatTurnFeedView } from "@/app/components/chat/ChatTurnFeed";
import {
  canPreviewAttachmentTarget,
  describeAttachmentPreview,
} from "@/lib/chatAttachmentPreview";
import {
  type ChatTurnEventPayload,
  feedAnswerText,
  hasFeedEntries,
} from "@/lib/chatTurnFeed";
import { toast } from "@/lib/toast";
import { formatRelativeTime } from "@/lib/time";
import { useDaemonStore } from "@/stores/daemonStore";
import { useChatStore } from "@/stores/chatStore";

function shouldUseFullWidth(text: string): boolean {
  const value = text.trim();
  return (
    value.length > 160 ||
    value.includes("\n") ||
    value.includes("```") ||
    value.includes("|") ||
    value.includes("\t")
  );
}

function formatTokenUsage(opts: {
  tokenIn?: number;
  tokenOut?: number;
  cachedInputTokens?: number;
}): string {
  const parts: string[] = [];
  if (typeof opts.tokenIn === "number") parts.push(`输入 ${opts.tokenIn}`);
  if (typeof opts.tokenOut === "number") parts.push(`输出 ${opts.tokenOut}`);
  if (typeof opts.cachedInputTokens === "number")
    parts.push(`缓存 ${opts.cachedInputTokens}`);
  return parts.join(" · ");
}

function formatAttachmentSize(sizeBytes?: number): string {
  if (typeof sizeBytes !== "number" || sizeBytes <= 0) return "";
  if (sizeBytes < 1024) return `${sizeBytes} B`;
  if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KB`;
  return `${(sizeBytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatAttachmentKind(kind?: string): string {
  switch ((kind ?? "").trim()) {
    case "image":
      return "图片";
    case "pdf":
      return "PDF";
    case "text":
      return "文本";
    default:
      return "附件";
  }
}

function guessPendingFileKind(file: File): string {
  const type = file.type.toLowerCase();
  const name = file.name.toLowerCase();
  if (type.startsWith("image/")) return "图片";
  if (type === "application/pdf" || name.endsWith(".pdf")) return "PDF";
  return "文本";
}

function fileIdentity(file: File): string {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

type ChatSessionsPageProps = {
  sessionId?: string;
};

type RuntimeSelectionMeta =
  | {
      expert_id?: string;
      provider?: string;
      cli_tool_id?: string;
    }
  | null
  | undefined;

type IdentityMeta =
  | {
      expert_id?: string;
      provider?: string;
      model?: string;
      cli_tool_id?: string;
    }
  | null
  | undefined;

type ChatRuntimeOption = {
  key: string;
  label: string;
  kind: "cli" | "sdk";
  provider: string;
  providers: string[];
  cliToolId?: string;
  runtimeConfigId: string;
  defaultModelId?: string;
};

type LiveUpdateBuffer = {
  streamingBySession: Record<string, string>;
  thinkingBySession: Record<string, string>;
  translatedThinkingDeltas: {
    sessionId: string;
    delta: string;
    entryId?: string;
  }[];
  thinkingTranslationStateBySession: Record<
    string,
    { applied: boolean; failed: boolean }
  >;
  turnEvents: ChatTurnEventPayload[];
};

function createLiveUpdateBuffer(): LiveUpdateBuffer {
  return {
    streamingBySession: {},
    thinkingBySession: {},
    translatedThinkingDeltas: [],
    thinkingTranslationStateBySession: {},
    turnEvents: [],
  };
}

function hasLiveUpdateBufferContent(buffer: LiveUpdateBuffer): boolean {
  return (
    buffer.turnEvents.length > 0 ||
    buffer.translatedThinkingDeltas.length > 0 ||
    Object.keys(buffer.streamingBySession).length > 0 ||
    Object.keys(buffer.thinkingBySession).length > 0 ||
    Object.keys(buffer.thinkingTranslationStateBySession).length > 0
  );
}

const sdkRuntimeLabels = {
  openai: "OpenAI SDK",
  anthropic: "Anthropic SDK",
} as const;

const codexReasoningEffortOptions = ["low", "medium", "high", "xhigh"] as const;
const defaultCodexReasoningEffort = "medium";

function normalizeCodexReasoningEffort(value?: string): string {
  const normalized = (value ?? "").trim().toLowerCase();
  return codexReasoningEffortOptions.includes(
    normalized as (typeof codexReasoningEffortOptions)[number],
  )
    ? normalized
    : defaultCodexReasoningEffort;
}

function normalizeIDList(values?: string[]): string[] {
  const next: string[] = [];
  const seen = new Set<string>();
  for (const value of values ?? []) {
    const trimmed = value.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    next.push(trimmed);
  }
  return next;
}

function toggleIDList(values: string[], target: string): string[] {
  const normalizedTarget = target.trim();
  if (!normalizedTarget) return normalizeIDList(values);
  const next = new Set(normalizeIDList(values));
  if (next.has(normalizedTarget)) next.delete(normalizedTarget);
  else next.add(normalizedTarget);
  return Array.from(next);
}

export function ChatSessionsPage(props: ChatSessionsPageProps) {
  const requestedSessionId = props.sessionId?.trim() || "";
  const daemonUrl = useDaemonStore((s) => s.daemonUrl);
  const health = useDaemonStore((s) => s.health);
  const experts = useDaemonStore((s) => s.experts);

  const sessions = useChatStore((s) => s.sessions);
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const messagesBySession = useChatStore((s) => s.messagesBySession);
  const streamingBySession = useChatStore((s) => s.streamingBySession);
  const thinkingBySession = useChatStore((s) => s.thinkingBySession);
  const translatedThinkingBySession = useChatStore(
    (s) => s.translatedThinkingBySession,
  );
  const thinkingTranslationStateBySession = useChatStore(
    (s) => s.thinkingTranslationStateBySession,
  );
  const turnMetaBySession = useChatStore((s) => s.turnMetaBySession);
  const turnInputByUserMessageId = useChatStore(
    (s) => s.turnInputByUserMessageId,
  );
  const usageByMessageId = useChatStore((s) => s.usageByMessageId);
  const activeTurnFeedBySession = useChatStore(
    (s) => s.activeTurnFeedBySession,
  );
  const completedTurnFeedByAssistantMessageId = useChatStore(
    (s) => s.completedTurnFeedByAssistantMessageId,
  );
  const loading = useChatStore((s) => s.loading);
  const sending = useChatStore((s) => s.sending);
  const error = useChatStore((s) => s.error);

  const setActiveSession = useChatStore((s) => s.setActiveSession);
  const applyLiveDeltaBatch = useChatStore((s) => s.applyLiveDeltaBatch);
  const setThinking = useChatStore((s) => s.setThinking);
  const setTranslatedThinking = useChatStore((s) => s.setTranslatedThinking);
  const clearStreaming = useChatStore((s) => s.clearStreaming);
  const clearThinking = useChatStore((s) => s.clearThinking);
  const resetThinkingTranslation = useChatStore(
    (s) => s.resetThinkingTranslation,
  );
  const setThinkingTranslationState = useChatStore(
    (s) => s.setThinkingTranslationState,
  );
  const setTurnMeta = useChatStore((s) => s.setTurnMeta);
  const setTurnInputMeta = useChatStore((s) => s.setTurnInputMeta);
  const setUsageMeta = useChatStore((s) => s.setUsageMeta);
  const startTurnFeed = useChatStore((s) => s.startTurnFeed);
  const completeTurnFeed = useChatStore((s) => s.completeTurnFeed);
  const clearTurnFeed = useChatStore((s) => s.clearTurnFeed);
  const refreshSessions = useChatStore((s) => s.refreshSessions);
  const loadMessages = useChatStore((s) => s.loadMessages);
  const loadTurns = useChatStore((s) => s.loadTurns);
  const createSession = useChatStore((s) => s.createSession);
  const sendTurn = useChatStore((s) => s.sendTurn);
  const forkSession = useChatStore((s) => s.forkSession);
  const archiveSession = useChatStore((s) => s.archiveSession);

  const [newTitle, setNewTitle] = useState("");
  const [newExpertId, setNewExpertId] = useState("");
  const [newSessionModalOpen, setNewSessionModalOpen] = useState(false);
  const [input, setInput] = useState("");
  const [turnExpertId, setTurnExpertId] = useState("");
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [dragActive, setDragActive] = useState(false);
  const [preview, setPreview] = useState<AttachmentPreviewState | null>(null);
  const [newModelId, setNewModelId] = useState("");
  const [turnModelId, setTurnModelId] = useState("");
  const [turnReasoningEffort, setTurnReasoningEffort] = useState("");
  const [cliTools, setCliTools] = useState<CLITool[]>([]);
  const [runtimeModelSettings, setRuntimeModelSettings] =
    useState<RuntimeModelSettings | null>(null);
  const [mcpSettings, setMCPSettings] = useState<MCPSettings | null>(null);
  const [newSessionMCPServerIDs, setNewSessionMCPServerIDs] = useState<
    string[]
  >([]);
  const [sessionMCPDraft, setSessionMCPDraft] = useState<string[]>([]);
  const messageScrollRef = useRef<HTMLDivElement | null>(null);
  const shouldAutoScrollRef = useRef(true);
  const liveUpdateBufferRef = useRef<LiveUpdateBuffer>(
    createLiveUpdateBuffer(),
  );
  const liveUpdateFrameRef = useRef<number | null>(null);
  const scrollFrameRef = useRef<number | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dragDepthRef = useRef(0);

  const activeSession = useMemo(
    () => sessions.find((s) => s.session_id === activeSessionId) ?? null,
    [sessions, activeSessionId],
  );
  const expertsById = useMemo(() => {
    const map = new Map<string, (typeof experts)[number]>();
    for (const e of experts) map.set(e.id, e);
    return map;
  }, [experts]);
  const selectableTools = useMemo(
    () => cliTools.filter((tool) => tool.enabled),
    [cliTools],
  );
  const toolsById = useMemo(() => {
    const map = new Map<string, CLITool>();
    for (const tool of selectableTools) map.set(tool.id, tool);
    return map;
  }, [selectableTools]);
  const runtimeConfigById = useMemo(() => {
    const map = new Map<string, RuntimeModelSettings["runtimes"][number]>();
    for (const runtime of runtimeModelSettings?.runtimes ?? [])
      map.set(runtime.id, runtime);
    return map;
  }, [runtimeModelSettings]);
  const isCodexToolId = useCallback(
    (toolId?: string) => {
      const normalized = toolId?.trim() || "";
      if (!normalized) return false;
      return (toolsById.get(normalized)?.cli_family || "").trim() === "codex";
    },
    [toolsById],
  );
  const runtimeOptions = useMemo(() => {
    const options: ChatRuntimeOption[] = [];
    for (const tool of selectableTools) {
      const runtime = runtimeConfigById.get(tool.id);
      if (!runtime) continue;
      options.push({
        key: `cli:${tool.id}`,
        label: tool.label,
        kind: "cli",
        provider: cliToolPrimaryProtocolFamily(tool),
        providers: Array.from(
          new Set(
            (runtime.models ?? [])
              .map((model) => (model.provider || "").trim())
              .filter(Boolean),
          ),
        ),
        cliToolId: tool.id,
        runtimeConfigId: runtime.id,
        defaultModelId: runtime.default_model_id,
      });
    }
    for (const provider of ["openai", "anthropic"] as const) {
      const runtimeId = provider === "openai" ? "sdk-openai" : "sdk-anthropic";
      const runtime = runtimeConfigById.get(runtimeId);
      if (!runtime || (runtime.models ?? []).length === 0) continue;
      options.push({
        key: `sdk:${provider}`,
        label: sdkRuntimeLabels[provider],
        kind: "sdk",
        provider,
        providers: [provider],
        runtimeConfigId: runtime.id,
        defaultModelId: runtime.default_model_id,
      });
    }
    return options;
  }, [runtimeConfigById, selectableTools]);
  const runtimeOptionsByKey = useMemo(() => {
    const map = new Map<string, ChatRuntimeOption>();
    for (const option of runtimeOptions) map.set(option.key, option);
    return map;
  }, [runtimeOptions]);
  const defaultSelectableRuntimeKey = runtimeOptions[0]?.key ?? "";
  const runtimeKeyForCLIFamily = useCallback(
    (cliFamily?: string) => {
      const normalized = (cliFamily || "").trim();
      if (!normalized) return "";
      for (const tool of selectableTools) {
        if ((tool.cli_family || "").trim() === normalized)
          return `cli:${tool.id}`;
      }
      return "";
    },
    [selectableTools],
  );
  const inferRuntimeKey = useCallback(
    (meta?: RuntimeSelectionMeta) => {
      const cliToolId = meta?.cli_tool_id?.trim() || "";
      if (cliToolId && runtimeOptionsByKey.has(`cli:${cliToolId}`))
        return `cli:${cliToolId}`;
      const explicit = meta?.expert_id?.trim() || "";
      if (explicit && runtimeOptionsByKey.has(`cli:${explicit}`))
        return `cli:${explicit}`;
      const expert = explicit ? expertsById.get(explicit) : undefined;
      const familyRuntimeKey = runtimeKeyForCLIFamily(expert?.cli_family);
      if (familyRuntimeKey && runtimeOptionsByKey.has(familyRuntimeKey))
        return familyRuntimeKey;
      const provider = (
        meta?.provider?.trim() ||
        expert?.provider ||
        ""
      ).toLowerCase();
      if (
        (provider === "openai" || provider === "anthropic") &&
        runtimeOptionsByKey.has(`sdk:${provider}`)
      ) {
        return `sdk:${provider}`;
      }
      return defaultSelectableRuntimeKey;
    },
    [
      defaultSelectableRuntimeKey,
      expertsById,
      runtimeKeyForCLIFamily,
      runtimeOptionsByKey,
    ],
  );
  const effectiveNewRuntimeKey =
    newExpertId && runtimeOptionsByKey.has(newExpertId)
      ? newExpertId
      : defaultSelectableRuntimeKey;
  const effectiveTurnRuntimeKey =
    turnExpertId && runtimeOptionsByKey.has(turnExpertId)
      ? turnExpertId
      : inferRuntimeKey(activeSession);
  const modelsForRuntime = useCallback(
    (runtimeKey: string) => {
      const runtime = runtimeOptionsByKey.get(runtimeKey);
      if (!runtime) return [] as RuntimeModelProfile[];
      return runtimeConfigById.get(runtime.runtimeConfigId)?.models ?? [];
    },
    [runtimeConfigById, runtimeOptionsByKey],
  );
  const effectiveNewModelId = useMemo(() => {
    const models = modelsForRuntime(effectiveNewRuntimeKey);
    if (newModelId && models.some((model) => model.id === newModelId))
      return newModelId;
    const runtime = runtimeOptionsByKey.get(effectiveNewRuntimeKey);
    if (
      runtime?.defaultModelId &&
      models.some((model) => model.id === runtime.defaultModelId)
    )
      return runtime.defaultModelId;
    return models[0]?.id ?? "";
  }, [
    effectiveNewRuntimeKey,
    modelsForRuntime,
    newModelId,
    runtimeOptionsByKey,
  ]);
  const effectiveTurnModelId = useMemo(() => {
    const models = modelsForRuntime(effectiveTurnRuntimeKey);
    if (turnModelId && models.some((model) => model.id === turnModelId))
      return turnModelId;
    if (
      activeSession?.model_id &&
      models.some((model) => model.id === activeSession.model_id)
    ) {
      return activeSession.model_id;
    }
    if (
      activeSession?.model &&
      models.some(
        (model) =>
          model.model === activeSession.model ||
          model.id === activeSession.model,
      )
    ) {
      return (
        models.find(
          (model) =>
            model.model === activeSession.model ||
            model.id === activeSession.model,
        )?.id ?? ""
      );
    }
    const runtime = runtimeOptionsByKey.get(effectiveTurnRuntimeKey);
    if (
      runtime?.defaultModelId &&
      models.some((model) => model.id === runtime.defaultModelId)
    )
      return runtime.defaultModelId;
    return models[0]?.id ?? "";
  }, [
    activeSession?.model,
    activeSession?.model_id,
    effectiveTurnRuntimeKey,
    modelsForRuntime,
    runtimeOptionsByKey,
    turnModelId,
  ]);
  const effectiveNewRuntime = runtimeOptionsByKey.get(effectiveNewRuntimeKey);
  const effectiveTurnRuntime = runtimeOptionsByKey.get(effectiveTurnRuntimeKey);
  const newSessionCliToolId =
    effectiveNewRuntime?.kind === "cli"
      ? effectiveNewRuntime.cliToolId?.trim() || ""
      : "";
  const activeSessionCliToolId =
    activeSession?.cli_tool_id?.trim() ||
    (effectiveTurnRuntime?.kind === "cli"
      ? effectiveTurnRuntime.cliToolId?.trim() || ""
      : "");
  const isCodexIdentity = useCallback(
    (meta?: IdentityMeta) => {
      const cliToolId = meta?.cli_tool_id?.trim() || "";
      if (cliToolId) return isCodexToolId(cliToolId);
      const runtime = runtimeOptionsByKey.get(inferRuntimeKey(meta));
      if (!runtime || runtime.kind !== "cli") return false;
      return isCodexToolId(runtime.cliToolId);
    },
    [inferRuntimeKey, isCodexToolId, runtimeOptionsByKey],
  );
  const effectiveTurnReasoningEffort = useMemo(
    () =>
      normalizeCodexReasoningEffort(
        turnReasoningEffort || activeSession?.reasoning_effort,
      ),
    [activeSession?.reasoning_effort, turnReasoningEffort],
  );
  const isTurnCodexRuntime = Boolean(
    effectiveTurnRuntime?.kind === "cli" &&
    isCodexToolId(effectiveTurnRuntime.cliToolId),
  );
  const selectableMCPServers = useCallback(
    (cliToolId?: string) => {
      const targetToolId = cliToolId?.trim() || "";
      if (!targetToolId) return [] as MCPServerSetting[];
      return mcpSettings?.servers ?? [];
    },
    [mcpSettings?.servers],
  );
  const defaultMCPServerIDs = useCallback(
    (cliToolId?: string) => {
      const targetToolId = cliToolId?.trim() || "";
      if (!targetToolId) return [] as string[];
      return selectableMCPServers(targetToolId)
        .filter((server) =>
          (server.default_enabled_cli_tool_ids ?? []).includes(targetToolId),
        )
        .map((server) => server.id);
    },
    [selectableMCPServers],
  );
  const sanitizeMCPSelection = useCallback(
    (ids: string[] | undefined, cliToolId?: string) => {
      const allowedIDs = new Set(
        selectableMCPServers(cliToolId).map((server) => server.id),
      );
      if (allowedIDs.size === 0) return [] as string[];
      return normalizeIDList(
        (ids ?? []).filter((id) => allowedIDs.has(id.trim())),
      );
    },
    [selectableMCPServers],
  );
  const newSessionMCPServers = useMemo(
    () => selectableMCPServers(newSessionCliToolId),
    [newSessionCliToolId, selectableMCPServers],
  );
  const normalizedNewSessionMCPServerIDs = useMemo(
    () => sanitizeMCPSelection(newSessionMCPServerIDs, newSessionCliToolId),
    [newSessionCliToolId, newSessionMCPServerIDs, sanitizeMCPSelection],
  );

  const formatModelIdentity = useCallback(
    (
      meta?: { expert_id?: string; provider?: string; model?: string } | null,
    ): string => {
      if (!meta) return "";
      const expertId = meta.expert_id?.trim() || "";
      const runtime = runtimeOptionsByKey.get(inferRuntimeKey(meta));
      const tool = runtime?.cliToolId
        ? toolsById.get(runtime.cliToolId)
        : expertId
          ? toolsById.get(expertId)
          : undefined;
      const expert = expertId ? expertsById.get(expertId) : undefined;
      const label =
        runtime?.kind === "cli"
          ? tool?.label || runtime.label || expert?.label || expertId
          : expert?.label || runtime?.label || tool?.label || expertId;
      const toolFamily = (tool?.cli_family || expert?.cli_family || "").trim();
      const provider =
        toolFamily === "iflow"
          ? "iflow"
          : (
              meta.provider?.trim() ||
              cliToolPrimaryProtocolFamily(tool) ||
              expert?.provider ||
              ""
            ).trim();
      const model = (meta.model?.trim() || expert?.model || "").trim();
      const parts: string[] = [];
      if (label) parts.push(label);
      if (model) {
        if (provider && provider !== "iflow")
          parts.push(`${provider}/${model}`);
        else parts.push(model);
      }
      return parts.join(" · ");
    },
    [expertsById, inferRuntimeKey, runtimeOptionsByKey, toolsById],
  );

  const selectSession = useCallback(
    (session: ChatSession | null, options?: { updateHash?: boolean }) => {
      const nextSessionId = session?.session_id ?? null;
      setActiveSession(nextSessionId);
      setTurnExpertId(session ? inferRuntimeKey(session) : "");
      setTurnModelId(session?.model_id || session?.model || "");
      setTurnReasoningEffort(session?.reasoning_effort || "");
      setSelectedFiles([]);
      if (options?.updateHash !== false) {
        goToChat(nextSessionId ?? undefined);
      }
    },
    [inferRuntimeKey, setActiveSession],
  );

  const appendSelectedFiles = useCallback((files: FileList | null) => {
    if (!files || files.length === 0) return;
    setSelectedFiles((prev) => {
      const seen = new Set(prev.map((file) => fileIdentity(file)));
      const next = [...prev];
      for (const file of Array.from(files)) {
        const identity = fileIdentity(file);
        if (seen.has(identity)) continue;
        seen.add(identity);
        next.push(file);
      }
      return next;
    });
  }, []);

  const removeSelectedFile = useCallback((targetIdentity: string) => {
    setSelectedFiles((prev) =>
      prev.filter((file) => fileIdentity(file) !== targetIdentity),
    );
  }, []);

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const closePreview = useCallback(() => {
    setPreview((prev) => {
      if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url);
      return null;
    });
  }, []);

  const openPreviewForFile = useCallback(async (file: File) => {
    const descriptor = describeAttachmentPreview(
      file.name,
      file.type,
      undefined,
    );
    if (descriptor.kind === "unsupported") return;
    setPreview((prev) => {
      if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url);
      return {
        name: file.name,
        kind: descriptor.kind,
        language: descriptor.language,
        loading:
          descriptor.kind === "code" ||
          descriptor.kind === "markdown" ||
          descriptor.kind === "text",
        revokeOnClose: false,
      };
    });
    if (descriptor.kind === "image" || descriptor.kind === "pdf") {
      const url = URL.createObjectURL(file);
      setPreview((prev) => {
        if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url);
        return {
          name: file.name,
          kind: descriptor.kind,
          url,
          revokeOnClose: true,
        };
      });
      return;
    }
    try {
      const content = await file.text();
      setPreview({
        name: file.name,
        kind: descriptor.kind,
        language: descriptor.language,
        content,
        revokeOnClose: false,
      });
    } catch (err) {
      setPreview({
        name: file.name,
        kind: descriptor.kind,
        error: err instanceof Error ? err.message : String(err),
        revokeOnClose: false,
      });
      toast({
        variant: "destructive",
        title: "附件预览失败",
        description: err instanceof Error ? err.message : String(err),
      });
    }
  }, []);

  const openPreviewForAttachment = useCallback(
    async (attachment: ChatAttachment) => {
      if (!activeSessionId) return;
      const descriptor = describeAttachmentPreview(
        attachment.file_name,
        attachment.mime_type,
        attachment.kind,
      );
      if (descriptor.kind === "unsupported") return;
      const contentUrl = chatAttachmentContentUrl(
        daemonUrl,
        activeSessionId,
        attachment.attachment_id,
      );
      setPreview((prev) => {
        if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url);
        return {
          name: attachment.file_name,
          kind: descriptor.kind,
          language: descriptor.language,
          url:
            descriptor.kind === "image" || descriptor.kind === "pdf"
              ? contentUrl
              : undefined,
          loading:
            descriptor.kind === "code" ||
            descriptor.kind === "markdown" ||
            descriptor.kind === "text",
          revokeOnClose: false,
        };
      });
      if (descriptor.kind === "image" || descriptor.kind === "pdf") {
        return;
      }
      try {
        const res = await fetch(contentUrl);
        if (!res.ok)
          throw new Error(`HTTP ${res.status} ${res.statusText}`.trim());
        const content = await res.text();
        setPreview({
          name: attachment.file_name,
          kind: descriptor.kind,
          language: descriptor.language,
          content,
          revokeOnClose: false,
        });
      } catch (err) {
        setPreview({
          name: attachment.file_name,
          kind: descriptor.kind,
          error: err instanceof Error ? err.message : String(err),
          revokeOnClose: false,
        });
        toast({
          variant: "destructive",
          title: "附件预览失败",
          description: err instanceof Error ? err.message : String(err),
        });
      }
    },
    [activeSessionId, daemonUrl],
  );

  useEffect(
    () => () => {
      if (preview?.revokeOnClose && preview.url)
        URL.revokeObjectURL(preview.url);
    },
    [preview],
  );

  const dragHasFiles = useCallback((event: DragEvent<HTMLDivElement>) => {
    return Array.from(event.dataTransfer?.types ?? []).includes("Files");
  }, []);

  const handleComposerDragEnter = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      if (!dragHasFiles(event)) return;
      event.preventDefault();
      event.stopPropagation();
      dragDepthRef.current += 1;
      setDragActive(true);
    },
    [dragHasFiles],
  );

  const handleComposerDragOver = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      if (!dragHasFiles(event)) return;
      event.preventDefault();
      event.stopPropagation();
      event.dataTransfer.dropEffect = "copy";
      if (!dragActive) setDragActive(true);
    },
    [dragActive, dragHasFiles],
  );

  const handleComposerDragLeave = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      if (!dragHasFiles(event)) return;
      event.preventDefault();
      event.stopPropagation();
      dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
      if (dragDepthRef.current === 0) setDragActive(false);
    },
    [dragHasFiles],
  );

  const handleComposerDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      if (!dragHasFiles(event)) return;
      event.preventDefault();
      event.stopPropagation();
      dragDepthRef.current = 0;
      setDragActive(false);
      appendSelectedFiles(event.dataTransfer.files);
    },
    [appendSelectedFiles, dragHasFiles],
  );

  useEffect(() => {
    let cancelled = false;
    void fetchCLIToolSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return;
        setCliTools(settings.tools ?? []);
      })
      .catch(() => {
        if (cancelled) return;
        setCliTools([]);
      });
    return () => {
      cancelled = true;
    };
  }, [daemonUrl]);

  useEffect(() => {
    let cancelled = false;
    void fetchRuntimeModelSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return;
        setRuntimeModelSettings(settings);
      })
      .catch(() => {
        if (cancelled) return;
        setRuntimeModelSettings(null);
      });
    return () => {
      cancelled = true;
    };
  }, [daemonUrl]);

  useEffect(() => {
    let cancelled = false;
    void fetchMCPSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return;
        setMCPSettings({
          ...settings,
          servers: settings.servers ?? [],
          tools: settings.tools ?? [],
        });
      })
      .catch(() => {
        if (cancelled) return;
        setMCPSettings(null);
      });
    return () => {
      cancelled = true;
    };
  }, [daemonUrl]);

  useEffect(() => {
    if (!newSessionCliToolId) {
      setNewSessionMCPServerIDs([]);
      return;
    }
    setNewSessionMCPServerIDs(defaultMCPServerIDs(newSessionCliToolId));
  }, [defaultMCPServerIDs, newSessionCliToolId]);

  useEffect(() => {
    if (!activeSession) {
      setSessionMCPDraft([]);
      return;
    }
    setSessionMCPDraft(
      sanitizeMCPSelection(
        activeSession.mcp_server_ids ??
          defaultMCPServerIDs(activeSessionCliToolId),
        activeSessionCliToolId,
      ),
    );
  }, [
    activeSession,
    activeSessionCliToolId,
    defaultMCPServerIDs,
    sanitizeMCPSelection,
  ]);

  const messages = useMemo(
    () => (activeSessionId ? (messagesBySession[activeSessionId] ?? []) : []),
    [activeSessionId, messagesBySession],
  );
  const streaming = activeSessionId
    ? (streamingBySession[activeSessionId] ?? "")
    : "";
  const thinking = activeSessionId
    ? (thinkingBySession[activeSessionId] ?? "")
    : "";
  const translatedThinking = activeSessionId
    ? (translatedThinkingBySession[activeSessionId] ?? "")
    : "";
  const activeTurnFeed = activeSessionId
    ? activeTurnFeedBySession[activeSessionId]
    : undefined;
  const thinkingTranslationState = activeSessionId
    ? (thinkingTranslationStateBySession[activeSessionId] ?? {
        applied: false,
        failed: false,
      })
    : { applied: false, failed: false };
  const displayedThinking =
    thinkingTranslationState.applied && !thinkingTranslationState.failed
      ? translatedThinking
      : thinking;
  const pendingThinkingTranslation =
    thinkingTranslationState.applied &&
    !thinkingTranslationState.failed &&
    !displayedThinking.trim() &&
    Boolean(thinking.trim());
  const lastAssistantMessageId = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i -= 1) {
      if (messages[i]?.role === "assistant")
        return messages[i]?.message_id ?? null;
    }
    return null;
  }, [messages]);
  const pendingAnswerText = feedAnswerText(activeTurnFeed) || streaming;
  const pendingAssistant =
    sending || hasFeedEntries(activeTurnFeed) || streaming.length > 0;

  const flushBufferedLiveUpdates = useCallback(() => {
    const buffer = liveUpdateBufferRef.current;
    if (!hasLiveUpdateBufferContent(buffer)) return;
    liveUpdateBufferRef.current = createLiveUpdateBuffer();
    applyLiveDeltaBatch({
      streamingBySession: buffer.streamingBySession,
      thinkingBySession: buffer.thinkingBySession,
      translatedThinkingDeltas: buffer.translatedThinkingDeltas,
      thinkingTranslationStateBySession:
        buffer.thinkingTranslationStateBySession,
      turnEvents: buffer.turnEvents,
    });
  }, [applyLiveDeltaBatch]);

  const scheduleBufferedLiveUpdates = useCallback(() => {
    if (liveUpdateFrameRef.current !== null) return;
    liveUpdateFrameRef.current = window.requestAnimationFrame(() => {
      liveUpdateFrameRef.current = null;
      flushBufferedLiveUpdates();
    });
  }, [flushBufferedLiveUpdates]);

  const scheduleScrollToBottom = useCallback(() => {
    if (!shouldAutoScrollRef.current) return;
    if (scrollFrameRef.current !== null) {
      window.cancelAnimationFrame(scrollFrameRef.current);
    }
    scrollFrameRef.current = window.requestAnimationFrame(() => {
      scrollFrameRef.current = null;
      const el = messageScrollRef.current;
      if (!el || !shouldAutoScrollRef.current) return;
      el.scrollTop = el.scrollHeight;
    });
  }, []);

  const refresh = useCallback(async () => {
    await refreshSessions(daemonUrl);
  }, [daemonUrl, refreshSessions]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!requestedSessionId) return;
    const target =
      sessions.find((session) => session.session_id === requestedSessionId) ??
      null;
    if (!target || activeSessionId === target.session_id) return;
    selectSession(target, { updateHash: false });
  }, [activeSessionId, requestedSessionId, selectSession, sessions]);

  useEffect(() => {
    if (!activeSessionId) return;
    void Promise.all([
      loadMessages(daemonUrl, activeSessionId),
      loadTurns(daemonUrl, activeSessionId),
    ]);
  }, [activeSessionId, daemonUrl, loadMessages, loadTurns]);

  useEffect(() => {
    scheduleScrollToBottom();
  }, [
    activeTurnFeed,
    messages,
    scheduleScrollToBottom,
    streaming,
    thinking,
    thinkingTranslationState.failed,
    translatedThinking,
  ]);

  useEffect(() => {
    const el = messageScrollRef.current;
    if (!el) return;
    const onScroll = () => {
      const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
      shouldAutoScrollRef.current = distanceToBottom < 64;
    };
    const onWheel = (event: WheelEvent) => {
      if (event.deltaY < 0) {
        shouldAutoScrollRef.current = false;
      }
    };
    onScroll();
    el.addEventListener("scroll", onScroll, { passive: true });
    el.addEventListener("wheel", onWheel, { passive: true });
    return () => {
      el.removeEventListener("scroll", onScroll);
      el.removeEventListener("wheel", onWheel);
    };
  }, [activeSessionId]);

  useEffect(() => {
    shouldAutoScrollRef.current = true;
    scheduleScrollToBottom();
  }, [activeSessionId, scheduleScrollToBottom]);

  useEffect(
    () => () => {
      if (liveUpdateFrameRef.current !== null) {
        window.cancelAnimationFrame(liveUpdateFrameRef.current);
        liveUpdateFrameRef.current = null;
      }
      if (scrollFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollFrameRef.current);
        scrollFrameRef.current = null;
      }
    },
    [],
  );

  useEffect(() => {
    const bufferLiveUpdate = (env: {
      type: string;
      payload?: unknown;
    }): boolean => {
      const buffer = liveUpdateBufferRef.current;
      if (env.type === "chat.turn.event") {
        const payload = env.payload as ChatTurnEventPayload | undefined;
        if (
          !payload?.session_id ||
          !payload.user_message_id ||
          !payload.entry_id
        ) {
          return true;
        }
        buffer.turnEvents.push(payload);
        scheduleBufferedLiveUpdates();
        return true;
      }
      if (env.type === "chat.turn.thinking.delta") {
        const payload = env.payload as
          | { session_id?: string; delta?: string }
          | undefined;
        if (!payload?.session_id || typeof payload.delta !== "string")
          return true;
        buffer.thinkingBySession[payload.session_id] =
          (buffer.thinkingBySession[payload.session_id] ?? "") + payload.delta;
        scheduleBufferedLiveUpdates();
        return true;
      }
      if (env.type === "chat.turn.thinking.translation.delta") {
        const payload = env.payload as
          | { session_id?: string; delta?: string; entry_id?: string }
          | undefined;
        if (!payload?.session_id || typeof payload.delta !== "string")
          return true;
        buffer.thinkingTranslationStateBySession[payload.session_id] = {
          applied: true,
          failed: false,
        };
        buffer.translatedThinkingDeltas.push({
          sessionId: payload.session_id,
          delta: payload.delta,
          entryId: payload.entry_id,
        });
        scheduleBufferedLiveUpdates();
        return true;
      }
      if (env.type === "chat.turn.delta") {
        const payload = env.payload as
          | { session_id?: string; delta?: string }
          | undefined;
        if (!payload?.session_id || typeof payload.delta !== "string")
          return true;
        buffer.streamingBySession[payload.session_id] =
          (buffer.streamingBySession[payload.session_id] ?? "") + payload.delta;
        scheduleBufferedLiveUpdates();
        return true;
      }
      return false;
    };

    return onWsEnvelope((env) => {
      if (bufferLiveUpdate(env)) return;
      flushBufferedLiveUpdates();

      if (env.type === "chat.turn.started") {
        const payload = env.payload as
          | {
              session_id?: string;
              user_message_id?: string;
              expert_id?: string;
              provider?: string;
              model?: string;
            }
          | undefined;
        if (!payload?.session_id) return;
        clearStreaming(payload.session_id);
        clearThinking(payload.session_id);
        resetThinkingTranslation(payload.session_id);
        clearTurnFeed(payload.session_id);
        setTurnMeta(payload.session_id, {
          expert_id: payload.expert_id,
          provider: payload.provider,
          model: payload.model,
        });
        if (
          typeof payload.user_message_id === "string" &&
          payload.user_message_id
        ) {
          startTurnFeed(payload.session_id, payload.user_message_id, {
            expert_id: payload.expert_id,
            provider: payload.provider,
            model: payload.model,
          });
        }
        return;
      }
      if (env.type === "chat.turn.thinking.translation.failed") {
        const payload = env.payload as { session_id?: string } | undefined;
        if (!payload?.session_id) return;
        setThinkingTranslationState(payload.session_id, {
          applied: true,
          failed: true,
        });
        return;
      }
      if (env.type === "chat.turn.completed") {
        const payload = env.payload as
          | {
              session_id?: string;
              user_message_id?: string;
              message?: {
                message_id?: string;
                token_in?: number;
                token_out?: number;
              };
              reasoning_text?: string;
              translated_reasoning_text?: string;
              thinking_translation_applied?: boolean;
              thinking_translation_failed?: boolean;
              model_input?: string;
              context_mode?: string;
              token_in?: number;
              token_out?: number;
              cached_input_tokens?: number;
            }
          | undefined;
        if (!payload?.session_id) return;
        if (
          typeof payload.reasoning_text === "string" &&
          payload.reasoning_text.trim()
        ) {
          setThinking(payload.session_id, payload.reasoning_text);
        }
        if (payload.thinking_translation_applied === true) {
          setThinkingTranslationState(payload.session_id, {
            applied: true,
            failed: payload.thinking_translation_failed === true,
          });
          if (payload.thinking_translation_failed !== true) {
            setTranslatedThinking(
              payload.session_id,
              payload.translated_reasoning_text ?? "",
            );
          }
        } else {
          resetThinkingTranslation(payload.session_id);
        }
        if (
          typeof payload.user_message_id === "string" &&
          payload.user_message_id &&
          typeof payload.model_input === "string" &&
          payload.model_input.trim()
        ) {
          setTurnInputMeta(payload.user_message_id, {
            model_input: payload.model_input,
            context_mode: payload.context_mode,
          });
        }
        const assistantMessageId =
          typeof payload.message?.message_id === "string"
            ? payload.message.message_id
            : undefined;
        if (assistantMessageId) {
          const tokenIn =
            typeof payload.message?.token_in === "number"
              ? payload.message.token_in
              : typeof payload.token_in === "number"
                ? payload.token_in
                : undefined;
          const tokenOut =
            typeof payload.message?.token_out === "number"
              ? payload.message.token_out
              : typeof payload.token_out === "number"
                ? payload.token_out
                : undefined;
          const cachedInputTokens =
            typeof payload.cached_input_tokens === "number"
              ? payload.cached_input_tokens
              : undefined;
          setUsageMeta(assistantMessageId, {
            token_in: tokenIn,
            token_out: tokenOut,
            cached_input_tokens: cachedInputTokens,
          });
          completeTurnFeed(payload.session_id, assistantMessageId, {
            thinking: payload.reasoning_text,
            translatedThinking: payload.translated_reasoning_text,
            translationFailed: payload.thinking_translation_failed === true,
          });
        }
        clearStreaming(payload.session_id);
        setTurnMeta(payload.session_id, null);
        void Promise.all([
          refreshSessions(daemonUrl),
          loadMessages(daemonUrl, payload.session_id),
          loadTurns(daemonUrl, payload.session_id),
        ]);
        return;
      }
      if (env.type === "chat.session.compacted") {
        void refreshSessions(daemonUrl);
      }
    });
  }, [
    clearStreaming,
    clearThinking,
    resetThinkingTranslation,
    setTurnMeta,
    startTurnFeed,
    flushBufferedLiveUpdates,
    scheduleBufferedLiveUpdates,
    setThinkingTranslationState,
    setThinking,
    setTranslatedThinking,
    setTurnInputMeta,
    setUsageMeta,
    completeTurnFeed,
    clearTurnFeed,
    daemonUrl,
    loadMessages,
    loadTurns,
    refreshSessions,
  ]);

  const buildRuntimeRequest = useCallback(
    (runtimeKey: string, modelId: string) => {
      const runtime = runtimeOptionsByKey.get(runtimeKey);
      const resolvedModelId = modelId.trim() || undefined;
      if (!runtime) {
        return {
          expertId: undefined,
          cliToolId: undefined,
          modelId: resolvedModelId,
        };
      }
      if (runtime.kind === "cli") {
        return {
          expertId: undefined,
          cliToolId: runtime.cliToolId?.trim() || undefined,
          modelId: resolvedModelId,
        };
      }
      return {
        expertId: resolvedModelId,
        cliToolId: undefined,
        modelId: resolvedModelId,
      };
    },
    [runtimeOptionsByKey],
  );

  const onCreate = async () => {
    try {
      const selection = buildRuntimeRequest(
        effectiveNewRuntimeKey,
        effectiveNewModelId,
      );
      const created = await createSession(daemonUrl, {
        title: newTitle.trim() || undefined,
        expert_id: selection.expertId,
        cli_tool_id: selection.cliToolId,
        model_id: selection.modelId,
        reasoning_effort:
          selection.cliToolId && isCodexToolId(selection.cliToolId)
            ? defaultCodexReasoningEffort
            : undefined,
        mcp_server_ids: sanitizeMCPSelection(
          newSessionMCPServerIDs,
          newSessionCliToolId,
        ),
      });
      setNewTitle("");
      setInput("");
      setSelectedFiles([]);
      setTurnExpertId(inferRuntimeKey(created));
      setTurnModelId(created.model_id || created.model || "");
      setTurnReasoningEffort(created.reasoning_effort || "");
      setSessionMCPDraft(
        created.mcp_server_ids ??
          sanitizeMCPSelection(newSessionMCPServerIDs, newSessionCliToolId),
      );
      toast({ title: "会话已创建", description: created.session_id });
      await loadMessages(daemonUrl, created.session_id);
    } catch (err: unknown) {
      toast({
        variant: "destructive",
        title: "创建会话失败",
        description: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const onSend = async () => {
    if (!activeSessionId) return;
    const text = input.trim();
    if (!text && selectedFiles.length === 0) return;
    const draftInput = input;
    const draftFiles = selectedFiles;
    setInput("");
    setSelectedFiles([]);
    try {
      const selection = buildRuntimeRequest(
        effectiveTurnRuntimeKey,
        effectiveTurnModelId,
      );
      const turnMCPServerIDs = sanitizeMCPSelection(
        sessionMCPDraft,
        selection.cliToolId || activeSessionCliToolId,
      );
      await sendTurn(
        daemonUrl,
        activeSessionId,
        text,
        selection.expertId,
        selection.cliToolId,
        selection.modelId,
        draftFiles,
        turnMCPServerIDs,
        selection.cliToolId && isCodexToolId(selection.cliToolId)
          ? effectiveTurnReasoningEffort
          : undefined,
      );
    } catch (err: unknown) {
      setInput(draftInput);
      setSelectedFiles(draftFiles);
      toast({
        variant: "destructive",
        title: "发送失败",
        description: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const onFork = async () => {
    if (!activeSessionId) return;
    try {
      const forked = await forkSession(daemonUrl, activeSessionId);
      selectSession(forked);
      toast({ title: "已分叉会话", description: forked.session_id });
    } catch (err: unknown) {
      toast({
        variant: "destructive",
        title: "分叉失败",
        description: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const onDeleteSession = async (sessionId: string) => {
    const session = sessions.find((s) => s.session_id === sessionId);
    const label = session?.title?.trim() || sessionId;
    if (
      !window.confirm(
        `确认删除会话「${label}」吗？\n\n删除按钮当前行为为归档（本地保留，不再显示在活跃列表）。`,
      )
    ) {
      return;
    }
    try {
      await archiveSession(daemonUrl, sessionId);
      if (activeSessionId === sessionId) {
        const nextActive =
          sessions.find(
            (item) => item.session_id !== sessionId && item.status === "active",
          ) ?? null;
        selectSession(nextActive);
      }
      toast({ title: "会话已删除（归档）", description: sessionId });
    } catch (err: unknown) {
      toast({
        variant: "destructive",
        title: "删除失败",
        description: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const visibleSessions = useMemo(
    () => sessions.filter((s) => s.status === "active"),
    [sessions],
  );

  const pendingMeta = activeSessionId
    ? (turnMetaBySession[activeSessionId] ?? null)
    : null;
  const pendingIdentity = useMemo(() => {
    if (pendingMeta) {
      const id = formatModelIdentity(pendingMeta);
      if (id) return id;
    }
    if (effectiveTurnRuntimeKey.trim()) {
      const runtime = runtimeOptionsByKey.get(effectiveTurnRuntimeKey);
      const model =
        modelsForRuntime(effectiveTurnRuntimeKey).find(
          (item) => item.id === effectiveTurnModelId,
        )?.model || "";
      const id = formatModelIdentity({
        expert_id:
          runtime?.kind === "sdk"
            ? effectiveTurnModelId.trim()
            : runtime?.cliToolId?.trim(),
        provider: runtime?.provider,
        model,
      });
      if (id) return id;
    }
    if (activeSession) {
      const id = formatModelIdentity({
        expert_id: activeSession.expert_id,
        provider: activeSession.provider,
        model: activeSession.model,
      });
      if (id) return id;
    }
    return "";
  }, [
    activeSession,
    effectiveTurnModelId,
    effectiveTurnRuntimeKey,
    formatModelIdentity,
    modelsForRuntime,
    pendingMeta,
    runtimeOptionsByKey,
  ]);

  const activeSessionIdentity = useMemo(() => {
    if (!activeSession) return "";
    return formatModelIdentity({
      expert_id: activeSession.expert_id,
      provider: activeSession.provider,
      model: activeSession.model,
    });
  }, [activeSession, formatModelIdentity]);

  if (health.status === "error") {
    return (
      <Alert
        color="danger"
        title="无法连接守护进程"
        description={health.message}
      />
    );
  }

  return (
    <>
      <WorkspacePortal target="sidebarHeader">
        <div className="mb-3 flex items-center justify-between gap-3 px-1">
          <div className="flex min-w-0 items-center gap-2">
            <div className="text-sm font-semibold">会话</div>
            <span className="flex h-6 min-w-6 items-center justify-center rounded-full bg-default-100 px-2 text-xs font-medium text-muted-foreground">
              {visibleSessions.length}
            </span>
          </div>

          <Button
            color="primary"
            size="sm"
            className="w-[25%] min-w-[86px] rounded-2xl"
            startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />}
            onPress={() => setNewSessionModalOpen(true)}
          >
            新建会话
          </Button>
        </div>
      </WorkspacePortal>

      <WorkspacePortal target="sidebarBody">
        <>
          {error ? (
            <Alert
              color="danger"
              title="加载失败"
              description={error}
              className="mb-3"
            />
          ) : null}

          <div className="space-y-2">
            {loading ? (
              <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
                加载中…
              </div>
            ) : visibleSessions.length === 0 ? (
              <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
                暂无会话
              </div>
            ) : (
              visibleSessions.map((s) => (
                <button
                  key={s.session_id}
                  className={`w-full rounded-[22px] border px-3 py-3 text-left transition ${
                    s.session_id === activeSessionId
                      ? "border-primary/50 bg-primary/5 shadow-sm"
                      : "border-transparent bg-background/40 hover:border-default-200 hover:bg-background/80"
                  }`}
                  onClick={() => {
                    selectSession(s);
                  }}
                >
                  <>
                    <div className="flex items-center gap-2">
                      <RuntimeIdentityMark
                        codex={isCodexIdentity(s)}
                        provider={s.provider}
                        cliFamily={
                          s.cli_tool_id
                            ? toolsById.get(s.cli_tool_id)?.cli_family || ""
                            : ""
                        }
                        className="h-4 w-4 shrink-0 text-muted-foreground"
                      />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-2">
                          <div className="min-w-0">
                            <div className="truncate text-sm font-medium">
                              {s.title}
                            </div>
                          </div>
                          <span
                            className="shrink-0 rounded-full p-1 text-muted-foreground transition-colors hover:bg-danger/10 hover:text-danger"
                            title="删除会话"
                            onClick={(event) => {
                              event.stopPropagation();
                              void onDeleteSession(s.session_id);
                            }}
                          >
                            <Trash2
                              className="h-3.5 w-3.5"
                              aria-hidden="true"
                              focusable="false"
                            />
                          </span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                      <span className="truncate">
                        {s.provider}/{s.model}
                      </span>
                      <span className="shrink-0">
                        {formatRelativeTime(s.updated_at)}
                      </span>
                    </div>
                  </>
                </button>
              ))
            )}
          </div>
        </>
      </WorkspacePortal>

      <WorkspacePortal target="headerMeta">
        <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          {activeSession ? (
            <>
              <span className="truncate">{activeSession.session_id}</span>
              {activeSessionIdentity ? (
                <span className="truncate">· {activeSessionIdentity}</span>
              ) : null}
            </>
          ) : (
            <span>左侧创建会话后即可开始对话</span>
          )}
        </div>
      </WorkspacePortal>

      <WorkspacePortal target="headerTitle">
        <div className="truncate text-base font-semibold">
          {activeSession ? activeSession.title : "请选择或创建会话"}
        </div>
      </WorkspacePortal>

      <WorkspacePortal target="headerActions">
        <div className="flex min-w-0 items-center justify-self-end gap-2">
          {activeSession ? (
            <Chip size="sm" variant="flat">
              {activeSession.status}
            </Chip>
          ) : null}
          <Button
            size="sm"
            variant="light"
            isDisabled={!activeSessionId}
            onPress={() => void onFork()}
          >
            分叉
          </Button>
        </div>
      </WorkspacePortal>

      <WorkspacePortal target="content">
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/30">
          <div
            ref={messageScrollRef}
            className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-8"
          >
            <div className="mx-auto flex w-full max-w-[880px] flex-col gap-5">
              {messages.length === 0 && !pendingAssistant ? (
                <div className="flex min-h-full flex-1 items-center justify-center py-16">
                  <div className="max-w-md rounded-[24px] border border-dashed bg-background/70 px-6 py-8 text-center">
                    <div className="text-base font-medium">开始新的对话</div>
                    <div className="mt-2 text-sm text-muted-foreground">
                      从左侧选择会话，或先创建一个新会话。
                    </div>
                  </div>
                </div>
              ) : null}

              {messages.map((m) => {
                const isUser = m.role === "user";
                const isAssistant = m.role === "assistant";
                const fullWidth = shouldUseFullWidth(m.content_text);
                const identity = formatModelIdentity({
                  expert_id: m.expert_id,
                  provider: m.provider,
                  model: m.model,
                });
                const inputMeta = isUser
                  ? turnInputByUserMessageId[m.message_id]
                  : undefined;
                const tokenUsage = isAssistant
                  ? formatTokenUsage({
                      tokenIn:
                        m.token_in ?? usageByMessageId[m.message_id]?.token_in,
                      tokenOut:
                        m.token_out ??
                        usageByMessageId[m.message_id]?.token_out,
                      cachedInputTokens:
                        usageByMessageId[m.message_id]?.cached_input_tokens,
                    })
                  : "";
                const contextModeLabel =
                  inputMeta?.context_mode === "anchor"
                    ? "上下文模式：Anchor 续写"
                    : inputMeta?.context_mode === "reconstructed"
                      ? "上下文模式：重建上下文"
                      : inputMeta?.context_mode === "demo"
                        ? "上下文模式：Demo"
                        : "";
                const completedFeed = isAssistant
                  ? completedTurnFeedByAssistantMessageId[m.message_id]
                  : undefined;
                const showThinkingDrawer =
                  isAssistant &&
                  m.message_id === lastAssistantMessageId &&
                  Boolean(displayedThinking.trim()) &&
                  !pendingAssistant &&
                  !completedFeed;
                return (
                  <div
                    key={m.message_id}
                    className={`flex ${isUser ? "justify-end" : "justify-start"}`}
                  >
                    <div
                      className={`rounded-[24px] px-4 py-3 ${
                        isUser
                          ? "border border-default-200 bg-default-100/90 shadow-sm"
                          : "border border-default-200/70 bg-background/80 shadow-sm"
                      } ${fullWidth ? "w-full" : isUser ? "max-w-[78%]" : "max-w-[90%]"}`}
                    >
                      <div className="mb-1 text-[11px] font-medium text-muted-foreground">
                        {isUser ? (
                          <>你{identity ? ` · ${identity}` : ""}</>
                        ) : (
                          <div className="flex items-center gap-1.5">
                            <RuntimeIdentityMark
                              codex={isCodexIdentity({
                                expert_id: m.expert_id,
                                provider: m.provider,
                                model: m.model,
                              })}
                              provider={m.provider}
                              className="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                            />
                            <span>{identity || "助手"}</span>
                          </div>
                        )}
                      </div>
                      {showThinkingDrawer ? (
                        <details className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                          <summary className="cursor-pointer select-none text-muted-foreground">
                            查看完整思考过程
                          </summary>
                          <div className="chat-markdown mt-2 text-xs text-muted-foreground">
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>
                              {displayedThinking}
                            </ReactMarkdown>
                          </div>
                        </details>
                      ) : null}
                      {isAssistant ? (
                        <div className="chat-markdown text-sm">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {m.content_text}
                          </ReactMarkdown>
                        </div>
                      ) : (
                        <div className="whitespace-pre-wrap text-sm">
                          {m.content_text}
                        </div>
                      )}
                      {completedFeed ? (
                        <details className="mt-3 rounded-md border border-dashed bg-muted/20 px-2 py-2">
                          <summary className="cursor-pointer select-none text-xs text-muted-foreground">
                            查看本轮过程详情
                          </summary>
                          <div className="mt-3">
                            <ChatTurnFeedView
                              feed={completedFeed}
                              identity={identity}
                              compact
                            />
                          </div>
                        </details>
                      ) : null}
                      {Array.isArray(m.attachments) &&
                      m.attachments.length > 0 ? (
                        <div className="mt-2 flex flex-wrap gap-1">
                          {m.attachments.map((attachment) => {
                            const sizeLabel = formatAttachmentSize(
                              attachment.size_bytes,
                            );
                            const kindLabel = formatAttachmentKind(
                              attachment.kind,
                            );
                            return (
                              <div
                                key={attachment.attachment_id}
                                className="flex items-center gap-1 rounded-full border bg-background/60 px-2 py-1 text-xs"
                              >
                                <span className="max-w-[200px] truncate">
                                  {attachment.file_name}
                                </span>
                                {kindLabel ? (
                                  <span className="text-muted-foreground">
                                    {kindLabel}
                                  </span>
                                ) : null}
                                {sizeLabel ? (
                                  <span className="text-muted-foreground">
                                    {sizeLabel}
                                  </span>
                                ) : null}
                                {canPreviewAttachmentTarget(
                                  attachment.file_name,
                                  attachment.mime_type,
                                  attachment.kind,
                                ) ? (
                                  <button
                                    type="button"
                                    className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                                    onClick={() =>
                                      void openPreviewForAttachment(attachment)
                                    }
                                    title="预览附件"
                                  >
                                    <Eye className="h-3 w-3" />
                                  </button>
                                ) : null}
                              </div>
                            );
                          })}
                        </div>
                      ) : null}
                      {isUser && inputMeta?.model_input ? (
                        <details className="mt-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                          <summary className="cursor-pointer select-none text-muted-foreground">
                            查看实际携带内容
                          </summary>
                          {contextModeLabel ? (
                            <div className="mt-2 text-[11px] text-muted-foreground">
                              {contextModeLabel}
                            </div>
                          ) : null}
                          <div className="mt-2 whitespace-pre-wrap break-words text-muted-foreground">
                            {inputMeta.model_input}
                          </div>
                        </details>
                      ) : null}
                      {isAssistant && tokenUsage ? (
                        <div className="mt-2 border-t pt-2 text-[11px] text-muted-foreground">
                          {tokenUsage}
                        </div>
                      ) : null}
                    </div>
                  </div>
                );
              })}

              {pendingAssistant ? (
                <div className="flex justify-start">
                  <div
                    className={`rounded-[24px] border border-dashed bg-background/80 px-4 py-3 shadow-sm ${
                      shouldUseFullWidth(pendingAnswerText || displayedThinking)
                        ? "w-full"
                        : "max-w-[90%]"
                    }`}
                  >
                    {activeTurnFeed && hasFeedEntries(activeTurnFeed) ? (
                      <ChatTurnFeedView
                        feed={activeTurnFeed}
                        pending
                        identity={pendingIdentity}
                      />
                    ) : (
                      <>
                        <div className="mb-1 flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
                          <RuntimeIdentityMark
                            codex={isCodexIdentity(pendingMeta)}
                            provider={pendingMeta?.provider}
                            className="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                          />
                          <span>
                            {pendingIdentity || "助手"} ·{" "}
                            {streaming ? "回复中" : "思考中"}
                          </span>
                        </div>
                        {displayedThinking.trim() ? (
                          <details className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                            <summary className="cursor-pointer select-none text-muted-foreground">
                              查看完整思考过程
                            </summary>
                            <div className="mt-2 whitespace-pre-wrap break-words text-xs text-muted-foreground">
                              {displayedThinking}
                            </div>
                          </details>
                        ) : pendingThinkingTranslation ? (
                          <div className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
                            正在翻译思考过程…
                          </div>
                        ) : null}
                        {streaming ? (
                          <div className="whitespace-pre-wrap break-words text-sm">
                            {streaming}
                          </div>
                        ) : (
                          <div className="text-sm text-muted-foreground">
                            正在思考…
                          </div>
                        )}
                      </>
                    )}
                  </div>
                </div>
              ) : null}
            </div>
          </div>

          <div className="shrink-0 bg-background/80 px-4 py-2.5 backdrop-blur md:px-8">
            <div className="mx-auto w-full max-w-[880px]">
              <div
                className={`rounded-[28px] border bg-background p-2 shadow-sm transition ${dragActive ? "border-primary bg-primary/5" : "border-default-200/80"}`}
                onDragEnter={handleComposerDragEnter}
                onDragOver={handleComposerDragOver}
                onDragLeave={handleComposerDragLeave}
                onDrop={handleComposerDrop}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  className="hidden"
                  onChange={(event) => {
                    appendSelectedFiles(event.target.files);
                    event.currentTarget.value = "";
                  }}
                />
                {selectedFiles.length > 0 ? (
                  <div className="mb-3 flex flex-wrap gap-2 rounded-2xl border bg-background/40 p-2">
                    {selectedFiles.map((file) => {
                      const identity = fileIdentity(file);
                      return (
                        <div
                          key={identity}
                          className="flex max-w-full items-center gap-1 rounded-full border px-2 py-1 text-xs text-foreground"
                        >
                          <span className="max-w-[180px] truncate">
                            {file.name}
                          </span>
                          <span className="text-muted-foreground">
                            {guessPendingFileKind(file)}
                          </span>
                          <span className="text-muted-foreground">
                            {formatAttachmentSize(file.size)}
                          </span>
                          {canPreviewAttachmentTarget(file.name, file.type) ? (
                            <button
                              type="button"
                              className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                              onClick={() => void openPreviewForFile(file)}
                              title="预览附件"
                            >
                              <Eye className="h-3 w-3" />
                            </button>
                          ) : null}
                          <button
                            type="button"
                            className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                            onClick={() => removeSelectedFile(identity)}
                            title="移除附件"
                          >
                            <X className="h-3 w-3" />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                ) : null}

                <div className="flex flex-col gap-2">
                  <textarea
                    value={input}
                    onChange={(event) => setInput(event.currentTarget.value)}
                    placeholder="输入消息或上传附件..."
                    disabled={!activeSessionId || sending}
                    aria-label="消息输入框"
                    className="min-h-[96px] w-full resize-none overflow-y-auto rounded-[22px] border border-default-200/80 bg-background px-4 py-3 text-sm text-foreground outline-none transition placeholder:text-muted-foreground focus:border-primary focus:ring-2 focus:ring-primary/20 disabled:cursor-not-allowed disabled:opacity-60"
                  />

                  <div className="relative flex items-center gap-2">
                    {/* CLI/SDK 圆形图标选择器（固定总宽，容纳展开动画） */}
                    <div className="flex w-[320px] shrink-0 items-center gap-1">
                      {runtimeOptions.map((runtime) => {
                        const isSelected =
                          effectiveTurnRuntimeKey === runtime.key;
                        const isDisabled = !activeSessionId || sending;
                        const isCodex =
                          runtime.kind === "cli" && runtime.cliToolId
                            ? isCodexToolId(runtime.cliToolId)
                            : false;
                        const isClaude =
                          runtime.kind === "cli" &&
                          (runtime.provider || "").trim() === "anthropic";
                        const isSdkOpenAI =
                          runtime.kind === "sdk" &&
                          runtime.provider === "openai";
                        const isSdkAnthropic =
                          runtime.kind === "sdk" &&
                          runtime.provider === "anthropic";
                        const cliFamily =
                          runtime.kind === "cli"
                            ? runtime.cliToolId
                              ? toolsById.get(runtime.cliToolId)?.cli_family ||
                                ""
                              : ""
                            : "";
                        const isIFlow =
                          cliFamily === "iflow" ||
                          (runtime.kind === "cli" &&
                            runtime.provider === "iflow");
                        const isOpenCode = cliFamily === "opencode";
                        return (
                          <button
                            key={runtime.key}
                            type="button"
                            title={runtime.label}
                            disabled={isDisabled}
                            onClick={() => {
                              if (!isDisabled) setTurnExpertId(runtime.key);
                            }}
                            style={{
                              transition:
                                "width 0.25s ease, background 0.2s ease, border-color 0.2s ease",
                            }}
                            className={`flex h-8 shrink-0 items-center overflow-hidden rounded-full border-2 ${
                              isSelected
                                ? "gap-1.5 px-2"
                                : "w-8 justify-center px-0"
                            } ${
                              isSelected
                                ? "border-blue-500 bg-blue-50 ring-2 ring-blue-400/50 dark:bg-blue-950/40"
                                : "border-default-200/80 bg-background hover:border-blue-400/60 hover:bg-blue-50/50 dark:hover:bg-blue-950/20"
                            } ${isDisabled ? "cursor-not-allowed opacity-40" : "cursor-pointer"}`}
                          >
                            <span className="flex h-4 w-4 shrink-0 items-center justify-center">
                              {isOpenCode ? (
                                <OpenCodeIcon className="h-4 w-4" />
                              ) : isIFlow ? (
                                <IFlowIcon className="h-4 w-4" />
                              ) : isCodex || isSdkOpenAI ? (
                                <OpenAIIcon className="h-4 w-4" />
                              ) : isClaude || isSdkAnthropic ? (
                                <AnthropicIcon className="h-4 w-4" />
                              ) : (
                                <span className="text-[10px] font-semibold leading-none text-muted-foreground">
                                  {(runtime.label || "?")
                                    .slice(0, 2)
                                    .toUpperCase()}
                                </span>
                              )}
                            </span>
                            <span
                              style={{
                                maxWidth: isSelected ? "100px" : "0px",
                                opacity: isSelected ? 1 : 0,
                                transition:
                                  "max-width 0.25s ease, opacity 0.2s ease",
                              }}
                              className="flex-1 overflow-hidden whitespace-nowrap text-center text-xs font-medium text-blue-700 dark:text-blue-300"
                            >
                              {runtime.label.replace(/\s*CLI\s*/i, "").trim() ||
                                runtime.label}
                            </span>
                          </button>
                        );
                      })}
                    </div>

                    {/* 模型选择 + 思考程度（绝对居中于整个输入区）*/}
                    <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
                      <div className="pointer-events-auto">
                        {/* 模型+思考程度 合并下拉 */}
                        <Dropdown placement="top">
                          <DropdownTrigger>
                            <Button
                              variant="flat"
                              size="sm"
                              endContent={
                                <ChevronUp className="h-3.5 w-3.5 shrink-0" />
                              }
                              isDisabled={
                                !activeSessionId ||
                                sending ||
                                modelsForRuntime(effectiveTurnRuntimeKey)
                                  .length === 0
                              }
                              className="h-8 w-[200px] shrink-0 justify-center rounded-full px-3 text-xs"
                            >
                              <span className="truncate">
                                {(() => {
                                  const selected = modelsForRuntime(
                                    effectiveTurnRuntimeKey,
                                  ).find((m) => m.id === effectiveTurnModelId);
                                  const modelLabel = selected
                                    ? selected.label || selected.id
                                    : "选择模型";
                                  return isTurnCodexRuntime
                                    ? `${modelLabel} · ${effectiveTurnReasoningEffort}`
                                    : modelLabel;
                                })()}
                              </span>
                            </Button>
                          </DropdownTrigger>
                          <DropdownMenu
                            aria-label="模型与思考程度选择"
                            classNames={{ base: "p-0", list: "p-0" }}
                            itemClasses={{
                              base: "p-0 data-[hover=true]:bg-transparent rounded-none",
                            }}
                          >
                            <DropdownItem
                              key="model-thinking-panel"
                              textValue="模型与思考程度"
                            >
                              <div className="flex min-w-[360px] divide-x divide-default-200">
                                {/* 左：模型列表 */}
                                <div className="flex flex-1 flex-col py-1">
                                  <div className="px-3 py-1.5 text-[11px] font-semibold text-muted-foreground">
                                    模型
                                  </div>
                                  {modelsForRuntime(
                                    effectiveTurnRuntimeKey,
                                  ).map((model) => (
                                    <button
                                      key={model.id}
                                      type="button"
                                      onClick={() => setTurnModelId(model.id)}
                                      className={`flex w-full flex-col px-3 py-1.5 text-left text-xs transition hover:bg-default-100 ${
                                        effectiveTurnModelId === model.id
                                          ? "bg-primary/8 font-medium text-primary"
                                          : "text-foreground"
                                      }`}
                                    >
                                      <span>{model.label || model.id}</span>
                                      <span className="text-[10px] text-muted-foreground">
                                        {model.model}
                                      </span>
                                    </button>
                                  ))}
                                </div>
                                {/* 右：思考程度（仅 Codex） */}
                                {isTurnCodexRuntime && (
                                  <div className="flex w-[110px] shrink-0 flex-col py-1">
                                    <div className="px-3 py-1.5 text-[11px] font-semibold text-muted-foreground">
                                      思考程度
                                    </div>
                                    {codexReasoningEffortOptions.map(
                                      (effort) => (
                                        <button
                                          key={effort}
                                          type="button"
                                          onClick={() =>
                                            setTurnReasoningEffort(effort)
                                          }
                                          className={`flex w-full items-center px-3 py-1.5 text-xs transition hover:bg-default-100 ${
                                            effectiveTurnReasoningEffort ===
                                            effort
                                              ? "bg-primary/8 font-medium text-primary"
                                              : "text-foreground"
                                          }`}
                                        >
                                          {effort}
                                        </button>
                                      ),
                                    )}
                                  </div>
                                )}
                              </div>
                            </DropdownItem>
                          </DropdownMenu>
                        </Dropdown>
                      </div>
                    </div>

                    {/* 添加附件 + 发送（绝对定位到最右侧）*/}
                    <div className="ml-auto flex shrink-0 items-center gap-2">
                      {/* 添加附件 */}
                      <Button
                        variant="flat"
                        size="sm"
                        radius="full"
                        className="h-8 shrink-0 px-3"
                        aria-label="添加附件"
                        title="添加附件"
                        isDisabled={!activeSessionId || sending}
                        onPress={openFilePicker}
                        startContent={<Plus className="h-3.5 w-3.5" />}
                      >
                        添加附件
                      </Button>

                      {/* 发送 */}
                      <Button
                        color="primary"
                        size="sm"
                        radius="full"
                        isIconOnly
                        className="h-8 min-w-8 shrink-0"
                        aria-label={sending ? "发送中" : "发送消息"}
                        title={sending ? "发送中…" : "发送消息"}
                        isLoading={sending}
                        isDisabled={
                          !activeSessionId ||
                          sending ||
                          (!input.trim() && selectedFiles.length === 0)
                        }
                        onPress={() => void onSend()}
                      >
                        <ArrowUp className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </div>

                {dragActive ? (
                  <div className="mt-2 px-1 text-xs text-primary">
                    释放鼠标即可添加附件
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </WorkspacePortal>

      <AttachmentPreviewModal preview={preview} onClose={closePreview} />

      <Modal
        isOpen={newSessionModalOpen}
        onOpenChange={setNewSessionModalOpen}
        classNames={{ base: "w-[550px] max-w-[550px] h-[550px] max-h-[550px]" }}
      >
        <ModalContent>
          {() => (
            <>
              <ModalHeader>新建会话</ModalHeader>
              <ModalBody className="space-y-3 overflow-y-auto">
                <div className="grid grid-cols-3 gap-2">
                  <Input
                    label="会话标题（可选）"
                    placeholder="留空自动生成"
                    value={newTitle}
                    onValueChange={setNewTitle}
                  />
                  <Select
                    aria-label="运行时"
                    label="运行时"
                    placeholder="选择运行时"
                    selectedKeys={
                      effectiveNewRuntimeKey
                        ? new Set([effectiveNewRuntimeKey])
                        : new Set()
                    }
                    onSelectionChange={(keys) => {
                      if (keys === "all") return;
                      const first = keys.values().next().value;
                      if (typeof first === "string") setNewExpertId(first);
                    }}
                    disallowEmptySelection
                    isDisabled={runtimeOptions.length === 0}
                  >
                    {runtimeOptions.map((runtime) => (
                      <SelectItem key={runtime.key}>{runtime.label}</SelectItem>
                    ))}
                  </Select>
                  <Select
                    aria-label="模型"
                    label="模型"
                    placeholder="选择模型"
                    selectedKeys={
                      effectiveNewModelId
                        ? new Set([effectiveNewModelId])
                        : new Set()
                    }
                    renderValue={() => {
                      const selected = modelsForRuntime(
                        effectiveNewRuntimeKey,
                      ).find((model) => model.id === effectiveNewModelId);
                      return selected
                        ? `${selected.label || selected.id} · ${selected.model}`
                        : "";
                    }}
                    onSelectionChange={(keys) => {
                      if (keys === "all") return;
                      const first = keys.values().next().value;
                      if (typeof first === "string") setNewModelId(first);
                    }}
                    disallowEmptySelection
                    isDisabled={
                      modelsForRuntime(effectiveNewRuntimeKey).length === 0
                    }
                  >
                    {modelsForRuntime(effectiveNewRuntimeKey).map((model) => (
                      <SelectItem key={model.id}>
                        {model.label || model.id} · {model.model}
                      </SelectItem>
                    ))}
                  </Select>
                </div>
                <div className="space-y-2">
                  <div className="text-sm font-medium">MCP 服务器</div>
                  {newSessionMCPServers.length > 0 ? (
                    <div className="grid grid-cols-2 gap-2">
                      {newSessionMCPServers.map((server) => {
                        const selected =
                          normalizedNewSessionMCPServerIDs.includes(server.id);
                        return (
                          <div
                            key={server.id}
                            className="flex items-center justify-between gap-3 rounded-xl border bg-background/70 px-3 py-2"
                          >
                            <div className="truncate text-sm font-medium">
                              {server.id}
                            </div>
                            <Switch
                              size="sm"
                              isSelected={selected}
                              onValueChange={() => {
                                setNewSessionMCPServerIDs((prev) =>
                                  sanitizeMCPSelection(
                                    toggleIDList(prev, server.id),
                                    newSessionCliToolId,
                                  ),
                                );
                              }}
                            />
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="rounded-xl border border-dashed bg-background/40 px-3 py-3 text-sm text-muted-foreground">
                      当前未添加任何 MCP
                    </div>
                  )}
                </div>
              </ModalBody>
              <ModalFooter>
                <Button
                  variant="light"
                  onPress={() => setNewSessionModalOpen(false)}
                >
                  取消
                </Button>
                <Button
                  color="primary"
                  isDisabled={runtimeOptions.length === 0}
                  onPress={() => {
                    void onCreate();
                    setNewSessionModalOpen(false);
                  }}
                >
                  创建
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </>
  );
}
