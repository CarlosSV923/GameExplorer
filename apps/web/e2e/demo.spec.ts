import { expect, test, type Page } from '@playwright/test'

// The demo end to end (RF-60 to RF-65): no login, the seeded library, and
// the sample files through the whole upload flow.

/** Moves inside the app: reloading would start the demo over (RF-60). */
async function goInApp(page: Page, path: string) {
  await page.evaluate((to) => {
    history.pushState({}, '', to)
    dispatchEvent(new PopStateEvent('popstate'))
  }, path)
}

test('opens on the seeded library, without login', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('note')).toContainText('Modo demo')
  // The first console by maker and year (spec §6).
  await expect(page.getByRole('heading', { level: 1, name: 'Nintendo 64' })).toBeVisible()
  await expect(page.getByText('4 juegos')).toBeVisible()
})

test('stores the Wii sample after its password', async ({ page }) => {
  await page.goto('/subidas')
  const sample = page.getByRole('listitem').filter({ hasText: 'Wii con contraseña' })
  await sample.getByRole('button', { name: 'Probar' }).click()

  const form = page.getByRole('dialog')
  await form.getByLabel('Nombre del juego').fill('Zelda Twilight Princess')
  await form.getByRole('button', { name: 'Subir', exact: true }).click()

  // The password is the sample's hint.
  const password = page.getByLabel('Contraseña del archivo')
  await expect(password).toBeVisible({ timeout: 15_000 })
  await password.fill('demo')
  await page.getByRole('button', { name: 'Desbloquear' }).click()

  await page.getByRole('link', { name: 'Completar' }).click({ timeout: 15_000 })
  await page.getByRole('button', { name: 'Guardar en la biblioteca' }).click()
  await expect(page).toHaveURL(/\/consolas\/wii\/\d+/)
  await expect(page.getByRole('heading', { name: 'Zelda Twilight Princess' })).toBeVisible()
})

test('uploads without console and assigns the whole entry (RF-07b, RF-27a)', async ({ page }) => {
  await page.goto('/subidas')
  const sample = page.getByRole('listitem').filter({ hasText: 'Sin consola, a No asignados' })
  await sample.getByRole('button', { name: 'Probar' }).click()
  const form = page.getByRole('dialog')
  await form.getByLabel('Nombre del juego').fill('Splatoon 3')
  await form.getByRole('button', { name: 'Subir', exact: true }).click()
  await expect(page.getByText('Movido a No asignados')).toBeVisible({ timeout: 15_000 })

  await goInApp(page, '/no-asignados')
  await page.getByRole('button', { name: 'Asignar Splatoon 3' }).click()
  const assign = page.getByRole('dialog')
  // The option cards hide their radio: the card is what gets tapped.
  await assign.getByText('Nintendo Switch', { exact: true }).click()
  await expect(assign.getByRole('radio', { name: /Nintendo Switch/ })).toBeChecked()
  await assign.getByRole('button', { name: 'Asignar', exact: true }).click()

  await page.getByRole('link', { name: 'Completar' }).click({ timeout: 15_000 })
  const base = page.getByRole('article').filter({ hasText: 'Splatoon 3 [v0].xci' })
  await base.getByRole('radio', { name: 'Juego base' }).check()
  const update = page.getByRole('article').filter({ hasText: 'Splatoon 3 [v2752512].nsp' })
  await update.getByRole('checkbox', { name: 'Guardar este archivo' }).uncheck()
  // One click saves, even before the preview of the final names arrives.
  await page.getByRole('button', { name: 'Guardar en la biblioteca' }).click()
  await expect(page).toHaveURL(/\/consolas\/switch\/\d+/)

  await goInApp(page, '/no-asignados')
  await expect(page.getByText('Splatoon 3 [v2752512].nsp')).toBeVisible()
})

test('adds a second disc and numbers the stored one (RF-08a)', async ({ page }) => {
  await page.goto('/subidas')
  // A file of the visitor's own: only its name and size are used (RF-63).
  await page
    .locator('input[type="file"]')
    .first()
    .setInputFiles({
      name: 'sotc-2.iso',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('x'),
    })
  const form = page.getByRole('dialog')
  await form.getByText('PlayStation 2', { exact: true }).click()
  await form.getByLabel('Nombre del juego').fill('Shadow of the Colossus')
  await form.getByRole('button', { name: 'Subir', exact: true }).click()

  await page.getByRole('link', { name: 'Completar' }).click({ timeout: 15_000 })
  // The stored disc becomes Disc 1 and the new one is suggested as Disc 2.
  await expect(page.getByText('Shadow of the Colossus.iso', { exact: true })).toBeVisible()
  await expect(
    page.getByText('ps2/Shadow of the Colossus/Shadow of the Colossus (Disc 1).iso'),
  ).toBeVisible()
  await expect(
    page.getByText('ps2/Shadow of the Colossus/Shadow of the Colossus (Disc 2).iso'),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Guardar en la biblioteca' }).click()
  await expect(page).toHaveURL(/\/consolas\/ps2\/\d+/)
  await expect(page.getByText('Shadow of the Colossus (Disc 2).iso')).toBeVisible()
})
