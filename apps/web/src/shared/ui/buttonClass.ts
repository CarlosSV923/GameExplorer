import { cx } from './cx'

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'danger-solid'
export type ButtonSize = 'lg' | 'md' | 'sm'

const base =
  'inline-flex items-center justify-center gap-2.5 rounded-full whitespace-nowrap select-none no-underline transition duration-120 ease-out cursor-pointer box-border'

const sizes: Record<ButtonSize, string> = {
  lg: 'h-control-lg px-7 text-body-lg',
  md: 'h-control-md px-5 text-body',
  sm: 'h-control-sm px-4 text-body-sm',
}

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-accent text-on-accent font-bold hover:bg-accent-hover hover:text-on-accent',
  secondary: 'border border-control bg-transparent text-ink-1 hover:bg-hover hover:text-ink-1',
  danger:
    'border border-danger bg-transparent text-danger font-bold hover:bg-hover hover:text-danger',
  'danger-solid': 'bg-danger text-on-accent font-bold hover:text-on-accent',
}

export const disabledStyle =
  'disabled:opacity-45 disabled:cursor-default enabled:active:scale-[.98]'

/** Class names of a button-looking element (also used by FileButton). */
export function buttonClass(variant: ButtonVariant = 'secondary', size: ButtonSize = 'md'): string {
  return cx(base, sizes[size], variants[variant])
}
