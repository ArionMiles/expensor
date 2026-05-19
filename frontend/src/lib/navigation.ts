import type { MessageKey } from '@/i18n/messages'

export interface NavigationTarget {
  id: string
  titleKey: MessageKey
  subtitleKey?: MessageKey
  descriptionKey: MessageKey
  path: string
  keywords?: string[]
}

export const NAVIGATION_TARGETS: NavigationTarget[] = [
  {
    id: 'dashboard',
    titleKey: 'nav.dashboard',
    descriptionKey: 'nav.dashboard.description',
    path: '/',
    keywords: ['home', 'summary', 'overview'],
  },
  {
    id: 'transactions',
    titleKey: 'nav.transactions',
    descriptionKey: 'nav.transactions.description',
    path: '/transactions',
    keywords: ['txns', 'ledger', 'spend'],
  },
  {
    id: 'setup',
    titleKey: 'nav.setup',
    descriptionKey: 'nav.setup.description',
    path: '/setup',
    keywords: ['setup', 'reader', 'connect'],
  },
  {
    id: 'rules',
    titleKey: 'nav.rules',
    descriptionKey: 'nav.rules.description',
    path: '/rules',
    keywords: ['regex', 'parser', 'filters'],
  },
  {
    id: 'diagnostics',
    titleKey: 'nav.diagnostics',
    descriptionKey: 'nav.diagnostics.description',
    path: '/diagnostics',
    keywords: ['diagnostics', 'failed extraction', 'regex'],
  },
  {
    id: 'expense-groups-categories',
    titleKey: 'nav.expenseGroups',
    subtitleKey: 'expenseGroups.tabs.categories',
    descriptionKey: 'nav.expenseGroups.categories.description',
    path: '/expense-groups?tab=categories',
    keywords: ['category', 'categories'],
  },
  {
    id: 'expense-groups-buckets',
    titleKey: 'nav.expenseGroups',
    subtitleKey: 'expenseGroups.tabs.buckets',
    descriptionKey: 'nav.expenseGroups.buckets.description',
    path: '/expense-groups?tab=buckets',
    keywords: ['bucket', 'buckets'],
  },
  {
    id: 'expense-groups-labels',
    titleKey: 'nav.expenseGroups',
    subtitleKey: 'expenseGroups.tabs.labels',
    descriptionKey: 'nav.expenseGroups.labels.description',
    path: '/expense-groups?tab=labels',
    keywords: ['label', 'labels', 'tags'],
  },
  {
    id: 'ignored-individual',
    titleKey: 'nav.ignored',
    subtitleKey: 'nav.ignored.individual.subtitle',
    descriptionKey: 'nav.ignored.individual.description',
    path: '/ignored?tab=individual',
    keywords: ['ignored', 'ignore', 'restore', 'individual'],
  },
  {
    id: 'ignored-merchant',
    titleKey: 'nav.ignored',
    subtitleKey: 'nav.ignored.merchant.subtitle',
    descriptionKey: 'nav.ignored.merchant.description',
    path: '/ignored?tab=merchant',
    keywords: ['ignored', 'ignore', 'merchant', 'merchant-level'],
  },
  {
    id: 'settings-general',
    titleKey: 'nav.settings',
    subtitleKey: 'nav.settings.general.subtitle',
    descriptionKey: 'nav.settings.general.description',
    path: '/settings?tab=general',
    keywords: ['settings', 'general', 'preferences'],
  },
  {
    id: 'settings-daemon',
    titleKey: 'nav.settings',
    subtitleKey: 'nav.settings.daemon.subtitle',
    descriptionKey: 'nav.settings.daemon.description',
    path: '/settings?tab=daemon',
    keywords: ['settings', 'daemon', 'scanner'],
  },
  {
    id: 'settings-sync',
    titleKey: 'nav.settings',
    subtitleKey: 'nav.settings.sync.subtitle',
    descriptionKey: 'nav.settings.sync.description',
    path: '/settings?tab=sync',
    keywords: ['settings', 'sync', 'community'],
  },
]
