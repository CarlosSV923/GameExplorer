import { createMemoryHistory } from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { createI18n } from '@/shared/i18n'
import { AppError } from '@/shared/kernel/errors'
import { fakeServices } from '@/test/fakeServices'

import { App } from './App'
import { safeRedirect } from './router'

function renderAt(path: string, services = fakeServices()) {
  const history = createMemoryHistory({ initialEntries: [path] })
  render(<App i18n={createI18n('es')} services={services} history={history} />)
  return { history, services }
}

describe('App', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('shows the carousel with the current console and its game count', async () => {
    renderAt('/')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Nintendo Switch' }),
    ).toBeInTheDocument()
    expect(screen.getByText('2 juegos')).toBeInTheDocument()
  })

  it('moves through the carousel with the arrow keys and remembers it in the URL', async () => {
    const user = userEvent.setup()
    const { history } = renderAt('/')
    await screen.findByRole('heading', { level: 1, name: 'Nintendo Switch' })

    await user.keyboard('{ArrowRight}')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'PlayStation 2' }),
    ).toBeInTheDocument()
    expect(screen.getByText('0 juegos')).toBeInTheDocument()
    expect(history.location.search).toContain('consola=ps2')

    await user.keyboard('{ArrowLeft}{ArrowLeft}')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Nintendo GameCube' }),
    ).toBeInTheDocument()
  })

  it('sends a signed-out visitor to the login and back after it', async () => {
    const user = userEvent.setup()
    const services = fakeServices()
    const session = vi
      .fn<typeof services.identity.session>()
      .mockRejectedValue(new AppError('unauthorized'))
    services.identity.session = session
    const { history } = renderAt('/ajustes/general', services)

    const password = await screen.findByLabelText('Contraseña')
    await user.type(password, 'wrong{Enter}')
    expect(
      await screen.findByText('Contraseña incorrecta. Inténtalo de nuevo.'),
    ).toBeInTheDocument()

    session.mockResolvedValue({ expiresAt: '2030-01-01T00:00:00Z' })
    await user.clear(password)
    await user.type(password, 'gameexplorer{Enter}')
    await waitFor(() => {
      expect(history.location.pathname).toBe('/ajustes/general')
    })
    expect(await screen.findByRole('heading', { level: 1, name: 'Ajustes' })).toBeInTheDocument()
  })

  it('switches the language from Settings › General and remembers it', async () => {
    const user = userEvent.setup()
    renderAt('/ajustes/general')
    await user.click(await screen.findByRole('button', { name: 'English' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument()
    expect(localStorage.getItem('ge.lang')).toBe('en')
  })

  it('shows the empty trash', async () => {
    renderAt('/ajustes/papelera')
    expect(await screen.findByText('La papelera está vacía')).toBeInTheDocument()
  })

  it('never redirects to another site after the login', () => {
    expect(safeRedirect('/consolas/ps2')).toBe('/consolas/ps2')
    expect(safeRedirect('//evil.example')).toBe('/')
    expect(safeRedirect('https://evil.example')).toBe('/')
    expect(safeRedirect(undefined)).toBe('/')
  })
})
