import type { CLITool, RuntimeModelProfile, RuntimeModelRuntime, RuntimeModelSettings } from '@/lib/daemon'

export type FlattenedRuntimeModel = RuntimeModelProfile & {
  runtime_id: string
  runtime_label: string
  kind: 'sdk' | 'cli'
  cli_tool_id?: string
}

export function runtimeIdForProvider(provider?: string | null): string {
  switch ((provider ?? '').trim()) {
    case 'openai':
      return 'sdk-openai'
    case 'anthropic':
      return 'sdk-anthropic'
    default:
      return ''
  }
}

export function runtimeIdForTool(tool?: Pick<CLITool, 'id' | 'cli_family'> | null): string {
  const explicit = (tool?.id ?? '').trim()
  if (explicit) return explicit
  switch ((tool?.cli_family ?? '').trim()) {
    case 'codex':
      return 'codex'
    case 'claude':
      return 'claude'
    case 'iflow':
      return 'iflow'
    case 'opencode':
      return 'opencode'
    default:
      return ''
  }
}

export function runtimeConfigById(
  settings: RuntimeModelSettings | null | undefined,
  runtimeId: string,
): RuntimeModelRuntime | undefined {
  const normalized = runtimeId.trim()
  if (!normalized) return undefined
  return settings?.runtimes?.find((item) => (item.id ?? '').trim() === normalized)
}

export function runtimeModelsForRuntime(
  settings: RuntimeModelSettings | null | undefined,
  runtimeId: string,
): RuntimeModelProfile[] {
  return runtimeConfigById(settings, runtimeId)?.models ?? []
}

export function runtimeDefaultModelId(
  settings: RuntimeModelSettings | null | undefined,
  runtimeId: string,
): string {
  return (runtimeConfigById(settings, runtimeId)?.default_model_id ?? '').trim()
}

export function runtimeModelsForToolId(
  settings: RuntimeModelSettings | null | undefined,
  toolId: string,
): RuntimeModelProfile[] {
  return runtimeModelsForRuntime(settings, toolId.trim())
}

export function runtimeDefaultModelForToolId(
  settings: RuntimeModelSettings | null | undefined,
  toolId: string,
): string {
  return runtimeDefaultModelId(settings, toolId.trim())
}

export function flattenRuntimeModels(
  settings: RuntimeModelSettings | null | undefined,
): FlattenedRuntimeModel[] {
  const out: FlattenedRuntimeModel[] = []
  for (const runtime of settings?.runtimes ?? []) {
    for (const model of runtime.models ?? []) {
      out.push({
        ...model,
        runtime_id: runtime.id,
        runtime_label: runtime.label,
        kind: runtime.kind,
        cli_tool_id: runtime.cli_tool_id,
      })
    }
  }
  return out
}
