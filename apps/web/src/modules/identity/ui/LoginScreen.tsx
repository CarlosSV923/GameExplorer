import { useEffect, useRef, useState, type SubmitEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { LanguageToggle } from '@/shared/i18n/LanguageToggle'
import { isAppError } from '@/shared/kernel/errors'
import { Button, GamepadIcon, HelpBar, TextField } from '@/shared/ui'

import { useLogin } from '../application/queries'

/** Carousel lines on both sides of the form, as on the home screen. */
function Rails({ side }: { side: 'left' | 'right' }) {
  const fade = side === 'left' ? 'bg-linear-to-r' : 'bg-linear-to-l'
  return (
    <div aria-hidden="true" className="hidden min-w-0 flex-1 flex-col gap-36 sm:flex">
      <span className={`h-0.5 ${fade} from-ink-1/75 from-70% to-transparent`} />
      <span className={`h-0.5 ${fade} from-ink-1/75 from-70% to-transparent`} />
    </div>
  )
}

export function LoginScreen({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation()
  const login = useLogin()
  const [password, setPassword] = useState('')
  const field = useRef<HTMLInputElement>(null)

  useEffect(() => {
    field.current?.focus()
  }, [])

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    if (!password) return
    login.mutate(password, {
      onSuccess,
      onError: () => {
        field.current?.select()
      },
    })
  }

  let error: string | undefined
  if (login.error) {
    if (isAppError(login.error, 'unauthorized')) error = t('login.wrongPassword')
    else if (isAppError(login.error, 'rateLimited')) error = t('login.tooManyAttempts')
    else error = t('errors.network')
  }

  return (
    <div className="flex min-h-app flex-col">
      <main className="flex flex-1 flex-col justify-center py-12">
        <div className="flex w-full items-center">
          <Rails side="left" />
          <form
            onSubmit={submit}
            className="box-border flex w-full max-w-[420px] flex-none flex-col gap-5.5 px-4"
          >
            <div className="mb-3 flex flex-col items-center gap-3 text-center">
              <GamepadIcon size={56} className="text-accent" strokeWidth={1.6} />
              <h1 className="m-0 text-display font-bold tracking-display uppercase">
                {t('app.name')}
              </h1>
              <p className="m-0 text-body-lg text-ink-2">{t('app.tagline')}</p>
            </div>
            <TextField
              ref={field}
              size="lg"
              type="password"
              autoComplete="current-password"
              label={t('login.password')}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
              }}
              {...(error ? { error } : {})}
            />
            <Button type="submit" variant="primary" size="lg" loading={login.isPending}>
              {t('login.submit')}
            </Button>
            <p className="m-0 text-center text-body-sm text-ink-3">{t('login.remembered')}</p>
          </form>
          <Rails side="right" />
        </div>
      </main>
      <HelpBar actions={[{ glyph: 'A', label: t('login.submit') }]} aside={<LanguageToggle />}>
        <span />
      </HelpBar>
    </div>
  )
}
