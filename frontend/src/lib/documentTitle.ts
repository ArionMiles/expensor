import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'

type Translator = (key: MessageKey) => string

type TitleRoute = {
  titleKey: MessageKey
  tabs?: {
    param: string
    defaultValue: string
    values: Record<string, MessageKey>
  }
}

const TITLE_ROUTES: Record<string, TitleRoute> = {
  '/': { titleKey: 'nav.dashboard' },
  '/transactions': { titleKey: 'nav.transactions' },
  '/setup': { titleKey: 'nav.setup' },
  '/rules': { titleKey: 'nav.rules' },
  '/rules/new/search': { titleKey: 'rules.emailSearch.title' },
  '/diagnostics': { titleKey: 'nav.diagnostics' },
  '/expense-groups': {
    titleKey: 'nav.expenseGroups',
    tabs: {
      param: 'tab',
      defaultValue: 'categories',
      values: {
        categories: 'expenseGroups.tabs.categories',
        buckets: 'expenseGroups.tabs.buckets',
        labels: 'expenseGroups.tabs.labels',
      },
    },
  },
  '/ignored': {
    titleKey: 'nav.ignored',
    tabs: {
      param: 'tab',
      defaultValue: 'merchant',
      values: {
        merchant: 'nav.ignored.merchant.subtitle',
        individual: 'nav.ignored.individual.subtitle',
      },
    },
  },
  '/settings': {
    titleKey: 'nav.settings',
    tabs: {
      param: 'tab',
      defaultValue: 'general',
      values: {
        general: 'nav.settings.general.subtitle',
        daemon: 'nav.settings.daemon.subtitle',
        sync: 'nav.settings.sync.subtitle',
        account: 'nav.settings.account.subtitle',
      },
    },
  },
}

function titleRouteForPath(pathname: string): TitleRoute | undefined {
  if (pathname === '/rules/new/search') return TITLE_ROUTES['/rules/new/search']
  if (pathname === '/rules/new' || pathname.startsWith('/rules/')) return TITLE_ROUTES['/rules']
  return TITLE_ROUTES[pathname]
}

export function resolveDocumentTitle(
  location: { pathname: string; search: string },
  t: Translator,
) {
  const route = titleRouteForPath(location.pathname)
  if (!route) return ''

  const pageTitle = t(route.titleKey)
  if (!route.tabs) return pageTitle

  const searchParams = new URLSearchParams(location.search)
  const rawTab = searchParams.get(route.tabs.param) ?? route.tabs.defaultValue
  const tabKey = route.tabs.values[rawTab] ?? route.tabs.values[route.tabs.defaultValue]

  return `${pageTitle} - ${t(tabKey)}`
}

export function DocumentTitle() {
  const location = useLocation()
  const { t } = useI18n()

  useEffect(() => {
    const title = resolveDocumentTitle(location, t)
    if (title) document.title = title
  }, [location, t])

  return null
}
