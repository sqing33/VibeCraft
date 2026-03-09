import type { CLITool, LLMModelProfile } from '@/lib/daemon'

function uniqueTrimmed(values?: string[]): string[] {
  const next: string[] = []
  const seen = new Set<string>()
  for (const value of values ?? []) {
    const trimmed = value.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    next.push(trimmed)
  }
  return next
}

function supportedProviders(tool: CLITool | undefined): string[] {
  if (!tool) return []
  const explicit = uniqueTrimmed(tool.protocol_families)
  if (explicit.length > 0) return explicit
  const single = (tool.protocol_family || '').trim()
  return single ? [single] : []
}

export function buildCLIToolModelProfiles(
  tool: CLITool | undefined,
  llmModels: LLMModelProfile[],
): LLMModelProfile[] {
  if (!tool) return []
  if ((tool.cli_family || '').trim() === 'iflow') {
    return uniqueTrimmed(tool.iflow_models).map((name) => ({
      id: name,
      label: name,
      provider: 'iflow',
      model: name,
      source_id: 'iflow',
    }))
  }
  const providers = new Set(supportedProviders(tool))
  if (providers.size === 0) return []
  return llmModels.filter((model) => providers.has((model.provider || '').trim()))
}

export function cliToolDefaultModelID(
  tool: CLITool | undefined,
  llmModels: LLMModelProfile[],
): string {
  if (!tool) return ''
  if ((tool.cli_family || '').trim() === 'iflow') {
    const options = buildCLIToolModelProfiles(tool, llmModels)
    const preferred = (tool.iflow_default_model || '').trim()
    if (preferred && options.some((item) => item.id === preferred)) return preferred
    return options[0]?.id ?? ''
  }
  return (tool.default_model_id || '').trim()
}
