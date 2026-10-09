import { createMemoryHistory } from '@tanstack/react-router'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { createI18n } from '@/shared/i18n'
import { AppError } from '@/shared/kernel/errors'
import type { UnassignedFile } from '@/modules/catalog/domain/types'
import { consoles, fakeServices } from '@/test/fakeServices'

import { App } from './App'
import { safeRedirect } from './router'

const unassignedFile: UnassignedFile = {
  id: 1,
  path: 'n64/Mario.z64',
  name: 'Mario.z64',
  origin: 'n64/',
  reason: 'samba',
  size: 8_000_000,
  arrivedAt: '2026-10-08T10:00:00Z',
  consoles: [],
  archive: false,
}

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

  it('moves through the consoles with games and the unassigned section', async () => {
    const user = userEvent.setup()
    const services = fakeServices()
    services.catalog.unassigned.list = vi.fn(() => Promise.resolve([unassignedFile]))
    const { history } = renderAt('/', services)
    await screen.findByRole('heading', { level: 1, name: 'Nintendo Switch' })

    await user.keyboard('{ArrowRight}')
    expect(await screen.findByRole('heading', { level: 1, name: 'Wii' })).toBeInTheDocument()
    expect(screen.getByText('1 juego')).toBeInTheDocument()
    expect(history.location.search).toContain('consola=wii')

    // PlayStation Portable has no games: the unassigned section comes next.
    await user.keyboard('{ArrowRight}')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'No asignados' }),
    ).toBeInTheDocument()
    expect(screen.getAllByText('1 archivo').length).toBeGreaterThan(0)

    await user.keyboard('{ArrowRight}')
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Nintendo Switch' }),
    ).toBeInTheDocument()
  })

  it('shows the empty library when no console has games', async () => {
    const services = fakeServices()
    services.catalog.consoles.list = vi.fn(() =>
      Promise.resolve(consoles.map((c) => ({ ...c, gameCount: 0 }))),
    )
    renderAt('/', services)
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Tu biblioteca está vacía' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/Nintendo Switch, Wii o PlayStation Portable/)).toBeInTheDocument()
  })

  it('asks for the name and console before uploading (RF-03)', async () => {
    const user = userEvent.setup()
    const services = fakeServices()
    const upload = vi.fn(() => ({ abort: vi.fn() }))
    services.ingestion.upload = upload
    const { history } = renderAt('/', services)
    await screen.findByRole('heading', { level: 1, name: 'Nintendo Switch' })

    const picker = document.querySelector<HTMLInputElement>('input[data-shortcut="upload"]')
    if (!picker) throw new Error('no upload input')
    await user.upload(picker, new File(['x'], 'Limbo.iso'))

    const dialog = await screen.findByRole('dialog', { name: 'Subir juego' })
    // The console in the middle of the carousel comes preselected.
    expect(within(dialog).getByRole('radio', { name: /Nintendo Switch/ })).toBeChecked()
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Nintendo Switch')

    await user.click(within(dialog).getByRole('radio', { name: /^Wii/ }))
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
    await user.type(within(dialog).getByRole('combobox', { name: 'Nombre del juego' }), 'Ōkami')
    expect(within(dialog).getByText('wii/Ōkami/')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Subir' }))
    await waitFor(() => {
      expect(history.location.pathname).toBe('/subidas')
    })
    expect(upload).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Limbo.iso' }),
      { spec: { console: 'wii', title: 'Ōkami' } },
      expect.anything(),
    )
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
