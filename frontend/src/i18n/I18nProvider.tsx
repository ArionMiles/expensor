import { createContext, useContext, useMemo } from 'react'
import { messages, type Locale, type MessageKey } from './messages'

type I18nContextValue = {
  locale: Locale
  t: (key: MessageKey, params?: Record<string, string | number>) => string
}

const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({
  locale = 'en',
  children,
}: {
  locale?: Locale
  children: React.ReactNode
}) {
  const value = useMemo(
    () => ({
      locale,
      t: (key: MessageKey, params?: Record<string, string | number>) => {
        const template = messages[locale][key] ?? key
        if (!params) return template
        return template.replace(/\{(\w+)\}/g, (match, name: string) =>
          Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
        )
      },
    }),
    [locale],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used within I18nProvider')
  return ctx
}
