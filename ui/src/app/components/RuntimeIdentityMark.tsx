import { AnthropicIcon } from './AnthropicIcon'
import { OpenAIIcon } from './OpenAIIcon'
import { OpenCodeIcon } from './OpenCodeIcon'
import { IFlowIcon } from './IFlowIcon'

type RuntimeIdentityMarkProps = {
  codex: boolean
  provider?: string
  cliFamily?: string
  className?: string
}

export function RuntimeIdentityMark(props: RuntimeIdentityMarkProps) {
  const cliFamily = (props.cliFamily || '').trim().toLowerCase()

  if (cliFamily === 'opencode') {
    return <OpenCodeIcon className={props.className} />
  }

  if (cliFamily === 'iflow' || (props.provider || '').trim().toLowerCase() === 'iflow') {
    return <IFlowIcon className={props.className} />
  }

  if (props.codex) {
    return <OpenAIIcon className={props.className} />
  }

  if ((props.provider || '').trim().toLowerCase() === 'anthropic') {
    return <AnthropicIcon className={props.className} />
  }

  return (
    <span
      aria-hidden="true"
      className={`inline-flex rounded-full border border-default-300/80 bg-transparent ${props.className ?? ''}`.trim()}
    />
  )
}
