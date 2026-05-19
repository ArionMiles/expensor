import { describe, expect, it } from 'vitest'
import {
  formatCurrencyForLocale,
  formatDateForLocale,
  formatMonthForLocale,
  formatNumberForLocale,
} from './format'

describe('i18n format helpers', () => {
  it('formats currency for the requested locale', () => {
    expect(formatCurrencyForLocale(1234.5, 'INR', 'en-IN')).toContain('1,234.50')
  })

  it('formats dates for the requested locale', () => {
    expect(formatDateForLocale(new Date('2026-04-28T00:00:00Z'), 'en-US')).toContain('Apr')
  })

  it('formats plain numbers for the requested locale', () => {
    expect(formatNumberForLocale(1234567, 'en-IN')).toBe('12,34,567')
  })

  it('formats month labels for the requested locale', () => {
    expect(formatMonthForLocale(new Date('2026-04-01T00:00:00Z'), 'en-US')).toBe('Apr')
  })
})
