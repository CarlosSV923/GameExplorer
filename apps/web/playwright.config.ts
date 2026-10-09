import { defineConfig, devices } from '@playwright/test'

// End-to-end tests against the demo build (RF-60): no backend needed, so CI
// runs them on every push. Desktop plus touch profiles (tablet and phone).
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL: 'http://localhost:4173',
    locale: 'es-ES',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'tablet', use: { ...devices['Galaxy Tab S4'] } },
    { name: 'phone', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: 'pnpm build && pnpm preview --port 4173 --strictPort',
    url: 'http://localhost:4173',
    env: { VITE_DATA_SOURCE: 'demo' },
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
})
