import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderWithProviders } from '@/test/render'
import { I18nProvider, useI18n } from './I18nProvider'

function Probe() {
  const { t } = useI18n()
  return <span>{t('nav.dashboard')}</span>
}

function IgnoredCopyProbe() {
  const { t } = useI18n()
  return (
    <>
      <span>{t('ignored.page.title')}</span>
      <span>{t('transactions.ignore.action')}</span>
      <span>{t('ignored.merchant.empty')}</span>
    </>
  )
}

describe('I18nProvider', () => {
  it('returns English messages by key', () => {
    renderWithProviders(
      <I18nProvider locale="en">
        <Probe />
      </I18nProvider>,
    )

    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('provides shared ignored transaction terminology', () => {
    renderWithProviders(
      <I18nProvider locale="en">
        <IgnoredCopyProbe />
      </I18nProvider>,
    )

    expect(screen.getByText('Ignored')).toBeInTheDocument()
    expect(screen.getByText('Ignore')).toBeInTheDocument()
    expect(screen.getByText('No merchant-wide ignore patterns yet.')).toBeInTheDocument()
  })
})
