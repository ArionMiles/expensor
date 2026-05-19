import { describe, expect, it } from 'vitest'
import { messages, type MessageKey } from '@/i18n/messages'
import { resolveDocumentTitle } from './documentTitle'

const t = (key: MessageKey) => messages.en[key]

describe('resolveDocumentTitle', () => {
  it('uses the page title for regular routes without the app name suffix', () => {
    expect(resolveDocumentTitle({ pathname: '/transactions', search: '' }, t)).toBe('Transactions')
  })

  it('uses page title followed by tab title for tabbed routes', () => {
    expect(resolveDocumentTitle({ pathname: '/settings', search: '?tab=sync' }, t)).toBe(
      'Settings - Community Sync',
    )
  })

  it('falls back to the page default tab when the tab query param is missing or invalid', () => {
    expect(resolveDocumentTitle({ pathname: '/expense-groups', search: '' }, t)).toBe(
      'Expense Groups - Categories',
    )
    expect(resolveDocumentTitle({ pathname: '/ignored', search: '?tab=unknown' }, t)).toBe(
      'Ignored - Merchant-wide',
    )
  })

  it('does not use the app name as a fallback title', () => {
    expect(resolveDocumentTitle({ pathname: '/unknown', search: '' }, t)).toBe('')
  })
})
