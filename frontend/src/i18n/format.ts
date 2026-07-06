export function formatCurrencyForLocale(amount: number, currency: string, locale: string) {
  if (!currency || currency.trim() === '') {
    return amount.toFixed(2)
  }
  try {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount)
  } catch {
    return `${currency} ${amount.toFixed(2)}`
  }
}

export function formatDateForLocale(value: Date, locale: string) {
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(value)
}

export function formatNumberForLocale(value: number, locale: string) {
  return new Intl.NumberFormat(locale).format(value)
}

export function formatMonthForLocale(value: Date, locale: string) {
  return new Intl.DateTimeFormat(locale, { month: 'short' }).format(value)
}

export function formatMonthYearForLocale(value: Date, locale: string) {
  return new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(value)
}
