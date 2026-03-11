import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/lib/utils'

type RainbowButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode
  startContent?: ReactNode
}

export function RainbowButton({ children, className, startContent, ...props }: RainbowButtonProps) {
  return (
    <button
      type="button"
      className={cn(
        'group relative inline-flex animate-rainbow cursor-pointer items-center justify-center gap-1.5 rounded-2xl border-0 bg-[length:200%] px-3 py-1.5 text-sm font-medium text-primary-foreground transition-colors',
        '[background-clip:padding-box,border-box,border-box] [background-origin:border-box]',
        '[border:calc(0.08*1rem)_solid_transparent]',
        'bg-[linear-gradient(hsl(var(--background)),hsl(var(--background))),linear-gradient(hsl(var(--background))_50%,rgba(255,255,255,0)_80%,rgba(0,0,0,0)),linear-gradient(90deg,var(--color-1),var(--color-5),var(--color-3),var(--color-4),var(--color-2))]',
        'dark:bg-[linear-gradient(hsl(var(--background)),hsl(var(--background))),linear-gradient(hsl(var(--background))_50%,rgba(0,0,0,0)_80%,rgba(0,0,0,0)),linear-gradient(90deg,var(--color-1),var(--color-5),var(--color-3),var(--color-4),var(--color-2))]',
        'text-foreground',
        className,
      )}
      style={{
        '--color-1': '#ff2975',
        '--color-2': '#A97CF8',
        '--color-3': '#F38CB8',
        '--color-4': '#FDCC92',
        '--color-5': '#ff2975',
        '--speed': '2s',
      } as React.CSSProperties}
      {...props}
    >
      {startContent ? <span className="shrink-0">{startContent}</span> : null}
      {children}
    </button>
  )
}
