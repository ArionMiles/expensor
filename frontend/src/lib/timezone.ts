const TIMEZONE_ALIASES: Record<string, string> = {
  'Asia/Kolkata': 'Asia/Calcutta',
}

export function normalizeTimezone(timezone?: string | null): string {
  const trimmed = timezone?.trim() ?? ''
  if (trimmed === '') return ''
  return TIMEZONE_ALIASES[trimmed] ?? trimmed
}

export function getBrowserTimezone(): string {
  return normalizeTimezone(Intl.DateTimeFormat().resolvedOptions().timeZone)
}

export function getTimezoneOptions(): string[] {
  const browserTimezone = getBrowserTimezone()
  const supportedTimezones = (
    Intl as unknown as { supportedValuesOf: (key: string) => string[] }
  ).supportedValuesOf('timeZone')
  return supportedTimezones.includes(browserTimezone)
    ? supportedTimezones
    : [...supportedTimezones, browserTimezone]
}
