import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export const SETTINGS_PANEL_BUTTON_CLASS = 'h-7 min-h-7 px-3 text-xs'
export const SETTINGS_TEXT_BUTTON_CLASS = 'min-w-0 h-6 min-h-6 px-1.5 text-xs'
export const SETTINGS_ICON_BUTTON_CLASS = 'h-7 min-h-7 min-w-7'

export const SETTINGS_INPUT_CLASSNAMES = {
  inputWrapper: 'h-8 min-h-8 px-3',
  input: 'text-sm',
}

export const SETTINGS_SELECT_CLASSNAMES = {
  trigger: 'h-8 min-h-8 px-3',
  value: 'text-sm',
}

export const SETTINGS_TEXTAREA_CLASSNAMES = {
  inputWrapper: 'rounded-3xl',
  input: 'text-sm',
}

export const SETTINGS_TABS_CLASSNAMES = {
  base: 'w-full shrink-0 min-h-0',
  panel: 'h-full min-h-0 flex-1 overflow-hidden pt-4',
  tabList: 'grid w-full shrink-0 grid-cols-8 rounded-full border bg-muted/40 p-1',
  tab: 'h-7 min-h-7 rounded-full px-3 text-xs',
  cursor: 'rounded-full shadow-none',
  tabContent: 'group-data-[selected=true]:text-primary-foreground',
}

type SettingsTabLayoutProps = {
  children: ReactNode
  footer?: ReactNode
  contentClassName?: string
}

export function SettingsTabLayout(props: SettingsTabLayoutProps) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-4">
      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        <div className={cn('space-y-4 pb-2', props.contentClassName)}>{props.children}</div>
      </div>
      {props.footer ? <SettingsFooter>{props.footer}</SettingsFooter> : null}
    </div>
  )
}

export function SettingsFooter(props: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'flex shrink-0 flex-wrap items-center justify-between gap-2 border-t bg-background/95 pt-2 backdrop-blur',
        props.className,
      )}
    >
      {props.children}
    </div>
  )
}
