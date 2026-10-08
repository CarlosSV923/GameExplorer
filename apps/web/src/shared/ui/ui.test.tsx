import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'

import { createI18n } from '../i18n'
import { InputProvider, useAction } from '../input'
import { Button, ChoiceChips, ConfirmDialog, HelpBar, ProgressBar, SearchField, TextField } from '.'

function wrap(ui: ReactNode) {
  return render(
    <I18nextProvider i18n={createI18n('es')}>
      <InputProvider>{ui}</InputProvider>
    </I18nextProvider>,
  )
}

function fakeGamepads(connected: boolean) {
  const pad = { buttons: [], axes: [] }
  Object.defineProperty(navigator, 'getGamepads', {
    configurable: true,
    value: () => (connected ? [pad] : []),
  })
  window.dispatchEvent(new Event(connected ? 'gamepadconnected' : 'gamepaddisconnected'))
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'getGamepads')
})

describe('Button', () => {
  it('ties a disabled reason to the button', () => {
    wrap(
      <Button disabled disabledReason="La consola tiene juegos.">
        Cambiar carpeta
      </Button>,
    )
    const button = screen.getByRole('button', { name: 'Cambiar carpeta' })
    expect(button).toBeDisabled()
    expect(button).toHaveAccessibleDescription('La consola tiene juegos.')
  })

  it('cannot be pressed twice while loading', () => {
    wrap(<Button loading>Guardar</Button>)
    expect(screen.getByRole('button', { name: /Guardar/ })).toBeDisabled()
  })
})

describe('fields', () => {
  it('marks a field invalid and describes the error', () => {
    wrap(<TextField label="Contraseña" type="password" error="Contraseña incorrecta." />)
    const field = screen.getByLabelText('Contraseña')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(field).toHaveAccessibleDescription('Contraseña incorrecta.')
  })

  it('focuses the search field with "/"', async () => {
    const user = userEvent.setup()
    wrap(<SearchField label="Buscar en la biblioteca" shortcut />)
    await user.keyboard('/')
    expect(screen.getByRole('searchbox', { name: 'Buscar en la biblioteca' })).toHaveFocus()
  })
})

describe('ChoiceChips', () => {
  function Kinds() {
    const [value, setValue] = useState<'base' | 'update'>('base')
    return (
      <ChoiceChips
        legend="Tipo"
        name="kind"
        value={value}
        onChange={setValue}
        options={[
          { value: 'base', label: 'Base' },
          { value: 'update', label: 'Update' },
        ]}
      />
    )
  }

  it('is a real radio group', async () => {
    const user = userEvent.setup()
    wrap(<Kinds />)
    expect(screen.getByRole('group', { name: 'Tipo' })).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: 'Update' }))
    expect(screen.getByRole('radio', { name: 'Update' })).toBeChecked()
  })
})

describe('ProgressBar', () => {
  it('exposes its value', () => {
    wrap(<ProgressBar value={61.6} label="Subida de pikmin4.zip" />)
    const bar = screen.getByRole('progressbar', { name: 'Subida de pikmin4.zip' })
    expect(bar).toHaveAttribute('aria-valuenow', '62')
  })
})

describe('ConfirmDialog', () => {
  function Trash() {
    const [open, setOpen] = useState(false)
    return (
      <>
        <button
          type="button"
          onClick={() => {
            setOpen(true)
          }}
        >
          Vaciar
        </button>
        {open && (
          <ConfirmDialog
            tone="danger"
            title="¿Vaciar la papelera?"
            confirmLabel="Vaciar papelera"
            onConfirm={() => {
              setOpen(false)
            }}
            onCancel={() => {
              setOpen(false)
            }}
          >
            No se puede deshacer.
          </ConfirmDialog>
        )}
      </>
    )
  }

  it('focuses the main action, makes the page inert and closes with Escape', async () => {
    const user = userEvent.setup()
    const { container } = wrap(<Trash />)
    const opener = screen.getByRole('button', { name: 'Vaciar' })
    await user.click(opener)

    const dialog = screen.getByRole('dialog', { name: '¿Vaciar la papelera?' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(screen.getByRole('button', { name: 'Vaciar papelera' })).toHaveFocus()
    expect(container).toHaveAttribute('inert')

    await user.keyboard('{Tab}')
    expect(screen.getByRole('button', { name: 'Cancelar' })).toHaveFocus()
    await user.keyboard('{Tab}')
    expect(screen.getByRole('button', { name: 'Vaciar papelera' })).toHaveFocus() // focus stays inside

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(container).not.toHaveAttribute('inert')
    expect(opener).toHaveFocus()
  })
})

describe('HelpBar', () => {
  const actions = [{ glyph: 'A', label: 'Seleccionar' }] as const

  it('shows buttons without a gamepad and glyphs with one', () => {
    wrap(
      <HelpBar actions={actions}>
        <button type="button">Menú</button>
      </HelpBar>,
    )
    expect(screen.getByRole('button', { name: 'Menú' })).toBeInTheDocument()
    expect(screen.queryByRole('img', { name: 'Botón A' })).not.toBeInTheDocument()

    act(() => {
      fakeGamepads(true)
    })
    expect(screen.getByRole('img', { name: 'Botón A' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Menú' })).not.toBeInTheDocument()

    act(() => {
      fakeGamepads(false)
    })
    expect(screen.getByRole('button', { name: 'Menú' })).toBeInTheDocument()
  })
})

describe('InputProvider', () => {
  function Screen({ onBack }: { onBack: () => void }) {
    useAction('back', () => {
      onBack()
      return true
    })
    return <input aria-label="Nombre" />
  }

  it('sends Escape to the newest handler and leaves arrows to text fields', async () => {
    const user = userEvent.setup()
    const outer = vi.fn()
    const inner = vi.fn()
    wrap(
      <>
        <Screen onBack={outer} />
        <Screen onBack={inner} />
      </>,
    )
    await user.keyboard('{Escape}')
    expect(inner).toHaveBeenCalledOnce()
    expect(outer).not.toHaveBeenCalled()

    const [field] = screen.getAllByRole('textbox', { name: 'Nombre' })
    field?.focus()
    await user.keyboard('{ArrowRight}')
    expect(field).toHaveFocus()
  })
})
