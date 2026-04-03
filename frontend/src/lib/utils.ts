import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatCurrency(amount: number, currency: string): string {
  if (!currency || currency.trim() === '') {
    return amount.toFixed(2)
  }
  try {
    return new Intl.NumberFormat('en-IN', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount)
  } catch {
    return `${currency} ${amount.toFixed(2)}`
  }
}

const READER_DISPLAY_NAMES: Record<string, string> = {
  gmail: 'Gmail',
  thunderbird: 'Thunderbird',
}

export function getReaderDisplayName(name: string): string {
  const lower = name.toLowerCase()
  if (READER_DISPLAY_NAMES[lower]) return READER_DISPLAY_NAMES[lower]
  return name
    .split(/[-_\s]+/)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ')
}

export function formatDate(isoString: string, includeTime = false): string {
  const date = new Date(isoString)
  const opts: Intl.DateTimeFormatOptions = {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    ...(includeTime ? { hour: '2-digit', minute: '2-digit', hour12: false } : {}),
  }
  return new Intl.DateTimeFormat('en-IN', opts).format(date)
}

export function formatDateShort(isoString: string): string {
  const date = new Date(isoString)
  return new Intl.DateTimeFormat('en-IN', {
    day: '2-digit',
    month: 'short',
  }).format(date)
}

export function formatRelative(isoString: string): string {
  const date = new Date(isoString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHr = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHr / 24)

  if (diffSec < 60) return 'just now'
  if (diffMin < 60) return `${diffMin}m ago`
  if (diffHr < 24) return `${diffHr}h ago`
  if (diffDay < 7) return `${diffDay}d ago`
  return formatDate(isoString)
}

// Bank brand colors keyed by a lowercase fragment present in the source string.
// Order matters: more specific fragments should come before broader ones.
const SOURCE_BRAND_COLORS: Array<{ fragment: string; color: string }> = [
  { fragment: 'hdfc', color: '#E31837' }, // HDFC Bank — red
  { fragment: 'icici', color: '#F47B20' }, // ICICI Bank — orange
  { fragment: 'sbi', color: '#22409A' }, // State Bank of India — blue
  { fragment: 'axis', color: '#97144D' }, // Axis Bank — burgundy
  { fragment: 'kotak', color: '#EE3124' }, // Kotak Mahindra — red-orange
  { fragment: 'indusind', color: '#5C2D91' }, // IndusInd Bank — purple
  { fragment: 'yes bank', color: '#003087' }, // Yes Bank — navy
  { fragment: 'idfc', color: '#007A4D' }, // IDFC First — green
]

const SOURCE_FALLBACK_COLORS = [
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#ec4899', // pink
  '#f59e0b', // amber
  '#06b6d4', // cyan
  '#14b8a6', // teal
]

export function getSourceColor(source: string): string {
  const lower = source.toLowerCase()
  for (const { fragment, color } of SOURCE_BRAND_COLORS) {
    if (lower.includes(fragment)) return color
  }
  let hash = 0
  for (let i = 0; i < source.length; i++) {
    hash = (hash << 5) - hash + source.charCodeAt(i)
    hash |= 0
  }
  return SOURCE_FALLBACK_COLORS[Math.abs(hash) % SOURCE_FALLBACK_COLORS.length]
}

const LABEL_COLORS = [
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#ec4899', // pink
  '#f59e0b', // amber
  '#06b6d4', // cyan
  '#d97706', // orange
  '#6366f1', // indigo
  '#14b8a6', // teal
]

export function getLabelColor(label: string): string {
  let hash = 0
  for (let i = 0; i < label.length; i++) {
    hash = (hash << 5) - hash + label.charCodeAt(i)
    hash |= 0
  }
  const index = Math.abs(hash) % LABEL_COLORS.length
  return LABEL_COLORS[index]
}

export function formatDuration(isoString: string): string {
  const start = new Date(isoString)
  const now = new Date()
  const diffMs = now.getTime() - start.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHr = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHr / 24)

  if (diffDay > 0) return `${diffDay}d ${diffHr % 24}h`
  if (diffHr > 0) return `${diffHr}h ${diffMin % 60}m`
  if (diffMin > 0) return `${diffMin}m`
  return `${diffSec}s`
}
