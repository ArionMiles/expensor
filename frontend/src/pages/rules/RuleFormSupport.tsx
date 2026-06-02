import { useRef, useState } from 'react'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { useI18n } from '@/i18n/I18nProvider'
import type { ReactNode } from 'react'

interface RegexResult {
  match: string | null
  invalid: boolean
}

export interface FormState {
  name: string
  subjectContains: string
  amountRegex: string
  merchantRegex: string
  currencyRegex: string
  sourceType: string
  bank: string
  senders: string[]
  senderDraft: string
}

export interface SampleState {
  name: string
  sender: string
  subject: string
  body: string
  expected: {
    amount: string
    merchant: string
    currency: string
  }
}

export interface FieldErrors {
  name?: string
  senders?: string
  amountRegex?: string
  merchantRegex?: string
  sampleSender?: string
}

export const RULE_CONTRIBUTION_GUIDE_URL =
  'https://github.com/ArionMiles/expensor/blob/main/.github/CONTRIBUTING.md#adding-bank-support'

export const emptyForm: FormState = {
  name: '',
  subjectContains: '',
  amountRegex: '',
  merchantRegex: '',
  currencyRegex: '',
  sourceType: '',
  bank: '',
  senders: [],
  senderDraft: '',
}

export function testRegex(pattern: string, body: string): RegexResult {
  if (!pattern) return { match: null, invalid: false }
  try {
    const match = new RegExp(pattern).exec(body)
    return { match: match?.[1] ?? null, invalid: false }
  } catch {
    return { match: null, invalid: true }
  }
}

export function diagnosticSample(
  diagnostic: {
    sender_email: string
    subject: string
    email_body: string
  },
  name: string,
): SampleState {
  return {
    name,
    sender: diagnostic.sender_email,
    subject: diagnostic.subject,
    body: diagnostic.email_body,
    expected: {
      amount: '',
      merchant: '',
      currency: '',
    },
  }
}

export function blankSample(name: string): SampleState {
  return {
    name,
    sender: '',
    subject: '',
    body: '',
    expected: {
      amount: '',
      merchant: '',
      currency: '',
    },
  }
}

export function sourceLabel(bank: string, sourceType: string) {
  return [bank.trim(), sourceType.trim()].filter(Boolean).join(' ')
}

export function uniqueSorted(values: string[]) {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort((a, b) =>
    a.localeCompare(b),
  )
}

export function slug(value: string) {
  return (
    value
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'rule'
  )
}

export function yamlScalar(value: string) {
  return JSON.stringify(value)
}

export function yamlNumberOrScalar(value: string) {
  const trimmed = value.trim()
  return /^-?\d+(?:\.\d+)?$/.test(trimmed) ? trimmed : yamlScalar(trimmed)
}

export function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

export function sampleHasValidationData(sample?: SampleState) {
  if (!sample) return false
  return Boolean(
    sample.sender.trim() ||
    sample.subject.trim() ||
    sample.body.trim() ||
    sample.expected.amount.trim() ||
    sample.expected.merchant.trim() ||
    sample.expected.currency.trim(),
  )
}

export function inputClasses(hasError = false, extra = '') {
  return [
    'mt-1 w-full rounded-lg border bg-input px-3 py-2 text-sm text-foreground',
    hasError ? 'border-destructive focus:border-destructive' : 'border-border',
    extra,
  ]
    .filter(Boolean)
    .join(' ')
}

type ZipEntry = {
  filename: string
  content: string
}

const textEncoder = new TextEncoder()
let crcTable: Uint32Array | null = null

export function downloadBlob(filename: string, blob: Blob) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function crc32(bytes: Uint8Array) {
  if (!crcTable) {
    crcTable = new Uint32Array(256)
    for (let n = 0; n < 256; n += 1) {
      let value = n
      for (let k = 0; k < 8; k += 1) {
        value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1
      }
      crcTable[n] = value >>> 0
    }
  }

  let crc = 0xffffffff
  for (const byte of bytes) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8)
  }
  return (crc ^ 0xffffffff) >>> 0
}

function writeUint16(bytes: number[], value: number) {
  bytes.push(value & 0xff, (value >>> 8) & 0xff)
}

function writeUint32(bytes: number[], value: number) {
  bytes.push(value & 0xff, (value >>> 8) & 0xff, (value >>> 16) & 0xff, (value >>> 24) & 0xff)
}

function writeBytes(bytes: number[], value: Uint8Array) {
  for (const byte of value) bytes.push(byte)
}

export function buildStoredZip(entries: ZipEntry[]) {
  const output: number[] = []
  const centralDirectory: number[] = []

  for (const entry of entries) {
    const filename = textEncoder.encode(entry.filename)
    const content = textEncoder.encode(entry.content)
    const checksum = crc32(content)
    const localHeaderOffset = output.length

    writeUint32(output, 0x04034b50)
    writeUint16(output, 20)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint32(output, checksum)
    writeUint32(output, content.length)
    writeUint32(output, content.length)
    writeUint16(output, filename.length)
    writeUint16(output, 0)
    writeBytes(output, filename)
    writeBytes(output, content)

    writeUint32(centralDirectory, 0x02014b50)
    writeUint16(centralDirectory, 20)
    writeUint16(centralDirectory, 20)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint32(centralDirectory, checksum)
    writeUint32(centralDirectory, content.length)
    writeUint32(centralDirectory, content.length)
    writeUint16(centralDirectory, filename.length)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint32(centralDirectory, 0)
    writeUint32(centralDirectory, localHeaderOffset)
    writeBytes(centralDirectory, filename)
  }

  const centralDirectoryOffset = output.length
  writeBytes(output, new Uint8Array(centralDirectory))
  writeUint32(output, 0x06054b50)
  writeUint16(output, 0)
  writeUint16(output, 0)
  writeUint16(output, entries.length)
  writeUint16(output, entries.length)
  writeUint32(output, centralDirectory.length)
  writeUint32(output, centralDirectoryOffset)
  writeUint16(output, 0)

  return new Blob([new Uint8Array(output)], { type: 'application/zip' })
}

type ComboboxProps = {
  label: string
  listboxLabel: string
  value: string
  options: string[]
  customValues: string[]
  onChange: (value: string) => void
  onAdd: (value: string) => void
  addLabel: (value: string) => string
}

export function SourceValueCombobox({
  label,
  listboxLabel,
  value,
  options,
  customValues,
  onChange,
  onAdd,
  addLabel,
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [readOnly, setReadOnly] = useState(true)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const allOptions = uniqueSorted([...options, ...customValues])
  const filtered = allOptions.filter((option) => option.toLowerCase().includes(value.toLowerCase()))
  const exactMatch = allOptions.some(
    (option) => option.toLowerCase() === value.trim().toLowerCase(),
  )
  const canAdd = value.trim() !== '' && filtered.length === 0 && !exactMatch
  const optionCount = filtered.length + (canAdd ? 1 : 0)

  const openMenu = () => {
    setOpen(true)
    navigation.resetHighlight()
  }

  const select = (nextValue: string) => {
    onChange(nextValue)
    setOpen(false)
    navigation.resetHighlight()
  }

  const add = () => {
    const nextValue = value.trim()
    if (!nextValue) return
    onAdd(nextValue)
    onChange(nextValue)
    setOpen(false)
    navigation.resetHighlight()
  }
  const navigation = useComboboxNavigation({
    open,
    optionCount,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selected = filtered[index]
      if (selected) select(selected)
      else if (canAdd && index === filtered.length) add()
    },
    onEnterWithoutSelection: () => {
      if (canAdd) add()
    },
  })
  const highlighted = navigation.highlightedIndex

  return (
    <div ref={containerRef}>
      <label className="mb-1 block text-sm text-muted-foreground" htmlFor={`${label}-input`}>
        {label}
      </label>
      <div className="relative">
        <input
          ref={inputRef}
          id={`${label}-input`}
          value={value}
          onChange={(event) => {
            onChange(event.target.value)
            openMenu()
          }}
          onFocus={() => {
            setReadOnly(false)
            openMenu()
          }}
          onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          readOnly={readOnly}
          autoComplete="off"
          data-1p-ignore="true"
          data-lpignore="true"
          data-form-type="other"
          aria-autocomplete="list"
          {...navigation.getComboboxProps({ listboxVisible: open && optionCount > 0 })}
          className="w-full rounded-lg border border-border bg-input px-3 py-2 pr-8 text-sm text-foreground outline-none transition-colors focus:border-primary"
        />
        <span
          aria-hidden="true"
          className="pointer-events-none absolute right-3 top-1/2 h-2 w-2 -translate-y-1/2 rotate-45 border-b-2 border-r-2 border-muted-foreground"
        />
      </div>
      <ComboboxListbox
        open={open && optionCount > 0}
        anchorRef={inputRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label={listboxLabel}
        onOpenChange={setOpen}
        className="rounded-lg p-1 text-sm text-card-foreground shadow-xl"
      >
        {filtered.map((option, index) => (
          <li
            key={option}
            {...navigation.getOptionProps(index, {
              selected: value === option,
              onMouseDown: () => select(option),
            })}
            className={comboboxOptionClass(
              index === highlighted,
              value === option,
              'rounded-md px-3 py-2 text-sm',
            )}
          >
            {option}
          </li>
        ))}
        {canAdd && (
          <li
            {...navigation.getOptionProps(filtered.length, {
              selected: false,
              onMouseDown: add,
            })}
            className={comboboxOptionClass(
              highlighted === filtered.length,
              false,
              'rounded-md px-3 py-2 text-sm font-medium text-primary',
            )}
          >
            {addLabel(value.trim())}
          </li>
        )}
      </ComboboxListbox>
    </div>
  )
}

export function ResultValue({
  result,
  optional = false,
}: {
  result: RegexResult
  optional?: boolean
}) {
  const { t } = useI18n()
  if (result.invalid) return <span className="text-destructive">{t('common.invalid')}</span>
  if (result.match !== null && result.match.trim() !== '') {
    return <span className="font-mono text-green-500">{result.match}</span>
  }
  if (optional) return <span className="text-muted-foreground">{t('common.optional')}</span>
  return <span className="text-destructive">{t('common.missing')}</span>
}

export function HintDot({ label, children }: { label: string; children: ReactNode }) {
  return (
    <span className="group relative inline-flex items-center">
      <button
        type="button"
        aria-label={label}
        className="h-2.5 w-2.5 rounded-full bg-amber-500 ring-4 ring-amber-500/15"
      />
      <span className="pointer-events-none absolute right-0 top-4 z-50 hidden w-56 rounded-lg border border-border bg-card p-2 text-xs normal-case leading-relaxed text-card-foreground shadow-xl ring-1 ring-border group-hover:block">
        {children}
      </span>
    </span>
  )
}
