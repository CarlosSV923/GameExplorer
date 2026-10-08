export const settingsTabs = ['consolas', 'papelera', 'general'] as const
export type SettingsTab = (typeof settingsTabs)[number]
