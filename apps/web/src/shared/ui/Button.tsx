import {
  useId,
  useRef,
  type AnchorHTMLAttributes,
  type ButtonHTMLAttributes,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { buttonClass, disabledStyle, type ButtonSize, type ButtonVariant } from './buttonClass'
import { cx } from './cx'
import { Spinner } from './Feedback'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: ReactNode
  loading?: boolean
  /** Why the button is disabled; shown next to it and announced with it. */
  disabledReason?: string
}

export function Button({
  variant = 'secondary',
  size = 'md',
  icon,
  loading = false,
  disabled,
  disabledReason,
  className,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  const reasonId = useId()
  const { t } = useTranslation()
  const isDisabled = Boolean(disabled) || loading
  const button = (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      aria-describedby={disabled && disabledReason ? reasonId : undefined}
      className={cx(buttonClass(variant, size), disabledStyle, className)}
      {...rest}
    >
      {loading ? <Spinner label={t('common.loading')} /> : icon}
      {children}
    </button>
  )
  if (!disabled || !disabledReason) return button
  return (
    <span className="inline-flex flex-col items-start gap-1">
      {button}
      <span id={reasonId} className="text-caption text-ink-3">
        {disabledReason}
      </span>
    </span>
  )
}

interface LinkButtonProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  href: string
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: ReactNode
}

/** A link that looks like a button: for navigation (Cancel, Review). */
export function LinkButton({
  variant = 'secondary',
  size = 'md',
  icon,
  className,
  children,
  ...rest
}: LinkButtonProps) {
  return (
    <a className={cx(buttonClass(variant, size), className)} {...rest}>
      {icon}
      {children}
    </a>
  )
}

interface IconButtonProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  'aria-label' | 'children'
> {
  /** Required: the button has no visible text. */
  label: string
  tone?: 'default' | 'danger'
  children: ReactNode
}

export function IconButton({
  label,
  tone = 'default',
  className,
  children,
  type = 'button',
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      className={cx(
        'inline-flex size-control-sm shrink-0 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent transition duration-120 ease-out hover:bg-hover disabled:cursor-default disabled:opacity-45',
        tone === 'danger' ? 'text-danger' : 'text-ink-1',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  )
}

interface FileButtonProps {
  onFiles: (files: File[]) => void
  multiple?: boolean
  accept?: string
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: ReactNode
  className?: string
  children: ReactNode
}

/**
 * The native file picker styled as a button: on touch devices it is the main
 * way to upload (RF-01). The input covers the label, so focus shows on it.
 */
export function FileButton({
  onFiles,
  multiple = true,
  accept,
  variant = 'primary',
  size = 'md',
  icon,
  className,
  children,
}: FileButtonProps) {
  const input = useRef<HTMLInputElement>(null)
  return (
    <label className={cx(buttonClass(variant, size), 'relative', className)}>
      {icon}
      {children}
      <input
        ref={input}
        type="file"
        multiple={multiple}
        accept={accept}
        className="absolute inset-0 cursor-pointer opacity-0"
        onChange={(e) => {
          const files = Array.from(e.currentTarget.files ?? [])
          if (files.length > 0) onFiles(files)
          if (input.current) input.current.value = '' // the same file can be picked again
        }}
      />
    </label>
  )
}
