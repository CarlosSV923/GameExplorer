import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { createI18n } from '@/shared/i18n'

import { App } from './App'

describe('App', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders in Spanish and switches to English', async () => {
    const user = userEvent.setup()
    render(<App i18n={createI18n('es')} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Componentes' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'EN' }))
    expect(screen.getByRole('heading', { level: 1, name: 'Components' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'EN' })).toHaveAttribute('aria-pressed', 'true')
    expect(localStorage.getItem('ge.lang')).toBe('en')
  })
})
