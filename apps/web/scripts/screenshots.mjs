// README screenshots and GIF, taken from the demo build (docs/tasks.md,
// phase 12). Run with `task screenshots`: it builds the demo, serves it and
// writes docs/screenshots/<lang>/*.png and upload.gif (ffmpeg converts the
// recorded video).
import { execFileSync } from 'node:child_process'
import { mkdirSync, readdirSync, renameSync, rmSync } from 'node:fs'
import { join } from 'node:path'

import { chromium, devices } from '@playwright/test'

const base = process.env.BASE_URL ?? 'http://localhost:4173'
const out = join(import.meta.dirname, '..', '..', '..', 'docs', 'screenshots')

const text = {
  en: {
    try: 'Try',
    name: 'Game name',
    upload: 'Upload',
    complete: 'Complete',
    base: 'Base game',
    update: 'Update',
    dlc: 'DLC',
    version: 'Version',
    dlcName: 'DLC name',
    replace: /^Replace/,
    store: /^Store/,
    sample: 'Switch: base, update and DLC',
  },
  es: {
    try: 'Probar',
    name: 'Nombre del juego',
    upload: 'Subir',
    complete: 'Completar',
    base: 'Juego base',
    update: 'Update',
    dlc: 'DLC',
    version: 'Versión',
    dlcName: 'Nombre del DLC',
    replace: /^Reemplazar/,
    store: /^Guardar/,
    sample: 'Switch: base, update y DLC',
  },
}

/** Moves inside the app: reloading would start the demo over. */
async function goInApp(page, path) {
  await page.evaluate((to) => {
    history.pushState({}, '', to)
    dispatchEvent(new PopStateEvent('popstate'))
  }, path)
  await page.waitForTimeout(800)
}

async function newPage(browser, lang, options = {}) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    ...options,
  })
  await context.addInitScript((l) => {
    localStorage.setItem('ge.lang', l)
  }, lang)
  return { context, page: await context.newPage() }
}

/** The Switch sample, from Uploads to its confirmation with every kind filled. */
async function confirmSample(page, t, pause) {
  await goInApp(page, '/subidas')
  await page
    .getByRole('listitem')
    .filter({ hasText: t.sample })
    .getByRole('button', { name: t.try })
    .click()
  await pause()
  const form = page.getByRole('dialog')
  await form.getByLabel(t.name).pressSequentially('Animal Crossing: New Horizons', { delay: 30 })
  await page.keyboard.press('Escape') // close the suggestions, keep the form
  await pause()
  await form.getByRole('button', { name: t.upload, exact: true }).click()
  await page.getByRole('link', { name: t.complete }).click({ timeout: 20_000 })
  await page.waitForTimeout(600)
  const cards = page.getByRole('article')
  await cards.nth(0).getByRole('radio', { name: t.base }).check()
  await cards
    .nth(0)
    .getByRole('radio', { name: t.replace })
    .check({ timeout: 5000 })
    .catch(() => undefined)
  await cards.nth(1).getByRole('radio', { name: t.update }).check()
  await cards.nth(1).getByLabel(t.version).fill('3.0.3')
  await cards.nth(2).getByRole('radio', { name: t.dlc }).check()
  await cards.nth(2).getByLabel(t.dlcName).fill('Happy Home Paradise')
  await page.waitForTimeout(900) // the preview of the final names
  // The duplicate base asks once the preview arrives.
  const replace = cards.nth(0).getByRole('radio', { name: t.replace })
  if (await replace.isVisible()) await replace.check()
  await page.waitForTimeout(600)
}

async function shoot(browser, lang) {
  const t = text[lang]
  const dir = join(out, lang)
  mkdirSync(dir, { recursive: true })

  // Desktop: carousel, game detail, upload confirmation, unassigned.
  const { context, page } = await newPage(browser, lang)
  await page.goto(base + '/')
  await page.waitForTimeout(1500)
  for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowRight') // to Switch
  await page.waitForTimeout(1200)
  await page.screenshot({ path: join(dir, 'home.png') })

  await goInApp(page, '/consolas/switch')
  await page
    .getByRole('link', { name: /Mario Kart 8 Deluxe/ })
    .first()
    .click()
  await page.waitForTimeout(1500)
  await page.screenshot({ path: join(dir, 'game.png') })

  await confirmSample(page, t, () => page.waitForTimeout(200))
  // A taller window rather than fullPage: the demo's notice is sticky.
  await page.setViewportSize({ width: 1280, height: 1320 })
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.waitForTimeout(400)
  await page.screenshot({ path: join(dir, 'confirm.png') })
  await page.setViewportSize({ width: 1280, height: 800 })

  await goInApp(page, '/no-asignados')
  await page.screenshot({ path: join(dir, 'unassigned.png') })
  await context.close()

  // Phone.
  const phone = await newPage(browser, lang, { ...devices['Pixel 7'] })
  await phone.page.goto(base + '/')
  await phone.page.waitForTimeout(1500)
  await goInApp(phone.page, '/consolas/switch')
  await phone.page.waitForTimeout(800)
  await phone.page.screenshot({ path: join(dir, 'phone.png') })
  await phone.context.close()

  // GIF: upload → confirm → stored game.
  const videoDir = join(dir, 'video')
  rmSync(videoDir, { recursive: true, force: true })
  const rec = await newPage(browser, lang, {
    recordVideo: { dir: videoDir, size: { width: 1280, height: 800 } },
  })
  await rec.page.goto(base + '/')
  await rec.page.waitForTimeout(1200)
  await confirmSample(rec.page, t, () => rec.page.waitForTimeout(700))
  await rec.page.getByRole('button', { name: t.store }).last().click()
  await rec.page.waitForURL(/\/consolas\/switch\/\d+/)
  await rec.page.waitForTimeout(2500)
  await rec.context.close()
  const video = readdirSync(videoDir).find((f) => f.endsWith('.webm'))
  if (!video) throw new Error('no video recorded')
  renameSync(join(videoDir, video), join(videoDir, 'upload.webm'))
  execFileSync('ffmpeg', [
    '-y',
    '-loglevel',
    'error',
    '-i',
    join(videoDir, 'upload.webm'),
    '-vf',
    'fps=10,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse=dither=bayer',
    join(dir, 'upload.gif'),
  ])
  rmSync(videoDir, { recursive: true, force: true })
}

const browser = await chromium.launch()
try {
  for (const lang of ['en', 'es']) await shoot(browser, lang)
} finally {
  await browser.close()
}
console.log('screenshots written to', out)
