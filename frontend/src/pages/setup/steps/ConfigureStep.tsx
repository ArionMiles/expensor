import { useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useSaveReaderConfig, useThunderbirdMailboxes, useThunderbirdProfiles } from '@/api/queries'
import type { ConfigField } from '@/api/types'
import { useFixedDropdownPosition } from '@/hooks/useFixedDropdownPosition'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'

// ─── Thunderbird profile combobox ─────────────────────────────────────────────

function ThunderbirdProfileField({
  id,
  label,
  value,
  onChange,
  disabled,
}: {
  id: string
  label: string
  value: string
  onChange: (v: string) => void
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: profiles = [], isLoading } = useThunderbirdProfiles()
  const { t } = useI18n()

  const filtered = profiles.filter((p) => p.toLowerCase().includes(value.toLowerCase()))
  const dropdownStyle = useFixedDropdownPosition(open && filtered.length > 0, inputRef)

  return (
    <div ref={containerRef} className="relative">
      <input
        id={id}
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        disabled={disabled}
        role="combobox"
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="listbox"
        placeholder={
          isLoading
            ? t('setup.configure.scanningProfiles')
            : t('setup.configure.profilePlaceholder')
        }
        className="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
      />
      {open &&
        filtered.length > 0 &&
        dropdownStyle &&
        createPortal(
          <ul
            role="listbox"
            aria-label={t('setup.configure.profileOptions')}
            style={dropdownStyle}
            className="fixed z-50 overflow-y-auto rounded-md border border-border bg-card shadow-lg"
          >
            {filtered.map((p) => (
              <li
                key={p}
                role="option"
                aria-selected={p === value}
                onMouseDown={() => {
                  onChange(p)
                  setOpen(false)
                }}
                className="cursor-pointer truncate px-3 py-1.5 text-xs text-foreground hover:bg-accent"
              >
                {p}
              </li>
            ))}
          </ul>,
          document.body,
        )}
      {!isLoading && profiles.length === 0 && (
        <p className="mt-0.5 text-xs text-muted-foreground">
          {t('setup.configure.noProfilesFound')}
        </p>
      )}
    </div>
  )
}

// ─── Thunderbird mailboxes multi-select ───────────────────────────────────────

function ThunderbirdMailboxesField({
  id,
  label,
  value,
  onChange,
  profilePath,
  disabled,
}: {
  id: string
  label: string
  value: string
  onChange: (v: string) => void
  profilePath: string
  disabled?: boolean
}) {
  const [input, setInput] = useState('')
  const [open, setOpen] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: available = [], isLoading } = useThunderbirdMailboxes(profilePath)
  const { t } = useI18n()

  const selected = value
    ? value
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
    : []

  const addMailbox = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed || selected.includes(trimmed)) return
    onChange([...selected, trimmed].join(','))
    setInput('')
    setOpen(false)
  }

  const removeMailbox = (name: string) => {
    onChange(selected.filter((s) => s !== name).join(','))
  }

  const filtered = available.filter(
    (m) => !selected.includes(m) && m.toLowerCase().includes(input.toLowerCase()),
  )
  const dropdownStyle = useFixedDropdownPosition(open && filtered.length > 0, inputRef)

  return (
    <div className="space-y-1.5">
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {selected.map((s) => (
            <span
              key={s}
              className="inline-flex items-center gap-1 rounded-sm border border-border bg-secondary px-1.5 py-0.5 text-xs text-foreground"
            >
              {s}
              <button
                type="button"
                onClick={() => removeMailbox(s)}
                className="text-muted-foreground hover:text-foreground"
                aria-label={t('setup.configure.removeMailbox', { mailbox: s })}
              >
                ✕
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="relative">
        <input
          id={id}
          ref={inputRef}
          value={input}
          onChange={(e) => {
            setInput(e.target.value)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 150)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              addMailbox(input)
            }
          }}
          disabled={disabled || !profilePath}
          role="combobox"
          aria-label={label}
          aria-expanded={open}
          aria-haspopup="listbox"
          placeholder={
            !profilePath
              ? t('setup.configure.selectProfileFirst')
              : isLoading
                ? t('setup.configure.loadingMailboxes')
                : t('setup.configure.mailboxPlaceholder')
          }
          className="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
        />
        {open &&
          filtered.length > 0 &&
          dropdownStyle &&
          createPortal(
            <ul
              role="listbox"
              aria-label={t('setup.configure.mailboxOptions')}
              style={dropdownStyle}
              className="fixed z-50 overflow-y-auto rounded-md border border-border bg-card shadow-lg"
            >
              {filtered.map((m) => (
                <li
                  key={m}
                  role="option"
                  aria-selected={selected.includes(m)}
                  onMouseDown={() => addMailbox(m)}
                  className="cursor-pointer px-3 py-1.5 text-xs text-foreground hover:bg-accent"
                >
                  {m}
                </li>
              ))}
            </ul>,
            document.body,
          )}
      </div>
      {profilePath && !isLoading && available.length === 0 && (
        <p className="text-xs text-muted-foreground">{t('setup.configure.noMailboxesFound')}</p>
      )}
    </div>
  )
}

// ─── ConfigureStep ────────────────────────────────────────────────────────────

interface ConfigureStepProps {
  readerName: string
  configSchema: ConfigField[]
  onNext: () => void
  onBack: () => void
}

export function ConfigureStep({ readerName, configSchema, onNext, onBack }: ConfigureStepProps) {
  const { t } = useI18n()
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    configSchema.forEach((field) => {
      init[field.name] = ''
    })
    return init
  })
  const [validationError, setValidationError] = useState<string | null>(null)
  const { mutate: saveConfig, isPending, error } = useSaveReaderConfig()

  const handleChange = (name: string, value: string) => {
    setValues((prev) => ({ ...prev, [name]: value }))
    setValidationError(null)
  }

  const handleSubmit = () => {
    for (const field of configSchema) {
      if (field.required && !values[field.name]?.trim()) {
        setValidationError(t('setup.configure.fieldRequired', { field: field.label }))
        return
      }
    }
    saveConfig({ readerName, config: values }, { onSuccess: () => onNext() })
  }

  const inputClass = cn(
    'w-full px-3 py-2 text-sm rounded-md',
    'bg-secondary border border-border text-foreground placeholder:text-muted-foreground',
    'focus:outline-none focus:ring-1 focus:ring-ring focus:border-primary',
  )

  if (configSchema.length === 0) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="mb-1 text-base font-semibold text-foreground">
            {t('setup.configure.title')}
          </h2>
          <p className="text-sm text-muted-foreground">{t('setup.configure.noneRequired')}</p>
        </div>
        <div className="flex items-center justify-between">
          <button
            onClick={onBack}
            className="px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {t('common.back')}
          </button>
          <button
            onClick={onNext}
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90"
          >
            {t('common.next')}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="mb-1 text-base font-semibold text-foreground">
          {t('setup.configure.title')}
        </h2>
        <p className="text-sm text-muted-foreground">
          {t('setup.configure.summaryPrefix')}{' '}
          <span className="font-mono text-primary">{readerName}</span>{' '}
          {t('setup.configure.summarySuffix')}
        </p>
      </div>

      <div className="space-y-4">
        {configSchema.map((field) => (
          <div key={field.name} className="space-y-1.5">
            <label
              htmlFor={`config-${field.name}`}
              className="block text-xs font-medium uppercase tracking-wider text-muted-foreground"
            >
              {field.label}
              {field.required && <span className="ml-1 text-destructive">*</span>}
            </label>

            {field.type === 'thunderbird-profile' ? (
              <ThunderbirdProfileField
                id={`config-${field.name}`}
                label={field.label}
                value={values[field.name] ?? ''}
                onChange={(v) => handleChange(field.name, v)}
              />
            ) : field.type === 'thunderbird-mailboxes' ? (
              <ThunderbirdMailboxesField
                id={`config-${field.name}`}
                label={field.label}
                value={values[field.name] ?? ''}
                onChange={(v) => handleChange(field.name, v)}
                profilePath={field.depends_on ? (values[field.depends_on] ?? '') : ''}
              />
            ) : field.type === 'textarea' ? (
              <textarea
                id={`config-${field.name}`}
                value={values[field.name] ?? ''}
                onChange={(e) => handleChange(field.name, e.target.value)}
                placeholder={field.placeholder}
                rows={4}
                className={cn(inputClass, 'resize-y')}
              />
            ) : (
              <input
                id={`config-${field.name}`}
                type={field.type === 'password' ? 'password' : 'text'}
                value={values[field.name] ?? ''}
                onChange={(e) => handleChange(field.name, e.target.value)}
                placeholder={field.placeholder}
                className={inputClass}
              />
            )}

            {field.help && <p className="text-xs text-muted-foreground">{field.help}</p>}
          </div>
        ))}
      </div>

      {(validationError || error) && (
        <p className="text-xs text-destructive" role="alert">
          {validationError ??
            (error instanceof Error ? error.message : t('setup.configure.saveFailed'))}
        </p>
      )}

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          {t('common.back')}
        </button>
        <button
          onClick={handleSubmit}
          disabled={isPending}
          className={cn(
            'rounded-md px-4 py-2 text-sm transition-colors',
            isPending
              ? 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50'
              : 'bg-primary text-primary-foreground hover:bg-primary/90',
          )}
        >
          {isPending ? t('common.saving') : t('setup.configure.saveAndContinue')}
        </button>
      </div>
    </div>
  )
}
