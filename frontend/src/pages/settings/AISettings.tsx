import { useEffect, useMemo, useRef, useState } from 'react'
import {
  useActivateLLMProvider,
  useDisconnectLLMProvider,
  useHealthCheckLLMProvider,
  useLLMProviderStatus,
  useLLMProviders,
  useSaveLLMProviderConfig,
  useSaveLLMProviderCredentials,
} from '@/api/queries'
import type { LLMProviderModelOption } from '@/api/types'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useTooltip } from '@/hooks/useTooltip'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'
import { AlertCircle, CheckCircle2, ExternalLink, KeyRound, Pencil, Unplug } from 'lucide-react'

const OPENAI_PROVIDER = 'openai'
const DEFAULT_MODEL = 'gpt-5.4-mini'
const DEFAULT_BASE_URL = 'https://api.openai.com/v1'
const OPENAI_API_DOCS_URL = 'https://developers.openai.com/api/reference/overview'

function StatusPill({ ready }: { ready: boolean }) {
  const { t } = useI18n()
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium',
        ready ? 'border-green-500/40 text-green-500' : 'border-warning/40 text-warning',
      )}
    >
      {ready ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
      {ready ? t('settings.ai.statusReady') : t('settings.ai.statusNeedsSetup')}
    </span>
  )
}

function resolveModelID(input: string, options: LLMProviderModelOption[], defaultModel: string) {
  const value = input.trim()
  if (!value) return defaultModel
  const match = options.find(
    (option) =>
      option.id.toLowerCase() === value.toLowerCase() ||
      option.display_name.toLowerCase() === value.toLowerCase(),
  )
  return match?.id ?? value
}

function displayModelValue(modelID: string, options: LLMProviderModelOption[]) {
  return options.find((option) => option.id === modelID)?.display_name ?? modelID
}

function ModelCombobox({
  value,
  options,
  onChange,
}: {
  value: string
  options: LLMProviderModelOption[]
  onChange: (value: string) => void
}) {
  const { t } = useI18n()
  const { handlers: chipTipHandlers, tip: chipTip } = useTooltip()
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const normalized = value.trim().toLowerCase()
  const filtered = options.filter((option) =>
    [option.display_name, option.id, option.quality, option.cost, option.description ?? '']
      .join(' ')
      .toLowerCase()
      .includes(normalized),
  )
  const exactMatch = options.some(
    (option) =>
      option.id.toLowerCase() === normalized || option.display_name.toLowerCase() === normalized,
  )
  const canUseCustom = value.trim() !== '' && !exactMatch
  const optionCount = filtered.length + (canUseCustom ? 1 : 0)

  const selectOption = (option: LLMProviderModelOption) => {
    onChange(option.display_name)
    setOpen(false)
    navigation.resetHighlight()
  }

  const selectCustom = () => {
    onChange(value.trim())
    setOpen(false)
    navigation.resetHighlight()
  }

  const navigation = useComboboxNavigation({
    open,
    optionCount,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const option = filtered[index]
      if (option) selectOption(option)
      else if (canUseCustom && index === filtered.length) selectCustom()
    },
    onEnterWithoutSelection: () => {
      if (canUseCustom) selectCustom()
    },
  })

  return (
    <div ref={containerRef}>
      <label className="block text-sm text-muted-foreground" htmlFor="openai-model-input">
        {t('settings.ai.model')}
      </label>
      <div className="relative mt-1">
        <input
          ref={inputRef}
          id="openai-model-input"
          value={value}
          onChange={(event) => {
            onChange(event.target.value)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          autoComplete="off"
          data-1p-ignore="true"
          data-lpignore="true"
          data-form-type="other"
          aria-autocomplete="list"
          {...navigation.getComboboxProps({ listboxVisible: open && optionCount > 0 })}
          className="w-full rounded-md border border-border bg-input px-3 py-2 pr-8 text-sm text-foreground outline-none transition-colors focus:border-primary focus:ring-1 focus:ring-ring"
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
        label={t('settings.ai.modelOptions')}
        onOpenChange={setOpen}
        className="rounded-lg p-1 text-sm text-card-foreground shadow-xl"
      >
        {filtered.map((option, index) => (
          <li
            key={option.id}
            {...navigation.getOptionProps(index, {
              selected: resolveModelID(value, options, '') === option.id,
              onMouseDown: () => selectOption(option),
            })}
            className={comboboxOptionClass(
              index === navigation.highlightedIndex,
              resolveModelID(value, options, '') === option.id,
              'rounded-md px-3 py-2 text-sm',
            )}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="font-medium text-foreground">{option.display_name}</div>
                {option.description && (
                  <div className="mt-0.5 text-xs text-muted-foreground">{option.description}</div>
                )}
              </div>
              <div className="flex shrink-0 gap-1">
                <span
                  className="rounded-full border border-primary/30 bg-primary/5 px-2 py-0.5 text-[10px] text-primary"
                  {...chipTipHandlers(t('settings.ai.qualityTooltip'))}
                >
                  {t('settings.ai.qualityChip', { value: option.quality })}
                </span>
                <span
                  className="rounded-full border border-amber-500/30 bg-amber-500/5 px-2 py-0.5 text-[10px] text-amber-600 dark:text-amber-300"
                  {...chipTipHandlers(t('settings.ai.costTooltip'))}
                >
                  {t('settings.ai.costChip', { value: option.cost })}
                </span>
              </div>
            </div>
          </li>
        ))}
        {canUseCustom && (
          <li
            {...navigation.getOptionProps(filtered.length, {
              selected: false,
              onMouseDown: selectCustom,
            })}
            className={comboboxOptionClass(
              navigation.highlightedIndex === filtered.length,
              false,
              'rounded-md px-3 py-2 text-sm font-medium text-primary',
            )}
          >
            {t('settings.ai.useCustomModel', { value: value.trim() })}
          </li>
        )}
      </ComboboxListbox>
      {chipTip}
    </div>
  )
}

export function AISettings() {
  const { t } = useI18n()
  const { handlers: disconnectTipHandlers, tip: disconnectTip } = useTooltip()
  const { data: providers = [], isLoading: providersLoading } = useLLMProviders()
  const provider = providers.find((candidate) => candidate.name === OPENAI_PROVIDER)
  const { data: status, isLoading: statusLoading } = useLLMProviderStatus(
    OPENAI_PROVIDER,
    Boolean(provider),
  )
  const saveConfig = useSaveLLMProviderConfig()
  const saveCredentials = useSaveLLMProviderCredentials()
  const healthcheck = useHealthCheckLLMProvider()
  const activate = useActivateLLMProvider()
  const disconnect = useDisconnectLLMProvider()

  const modelOptions = useMemo(() => provider?.model_options ?? [], [provider?.model_options])
  const defaultModel =
    modelOptions.find((option) => option.recommended)?.id || modelOptions[0]?.id || DEFAULT_MODEL

  const [modelInput, setModelInput] = useState(displayModelValue(defaultModel, modelOptions))
  const [baseURL, setBaseURL] = useState(DEFAULT_BASE_URL)
  const [baseURLEditing, setBaseURLEditing] = useState(false)
  const [apiKey, setAPIKey] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [confirmDisconnect, setConfirmDisconnect] = useState(false)
  const [activeAction, setActiveAction] = useState<'save' | 'test' | null>(null)
  const baseURLRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!status?.config) return
    setModelInput(displayModelValue(status.config.model || defaultModel, modelOptions))
    setBaseURL(status.config.base_url || DEFAULT_BASE_URL)
  }, [defaultModel, modelOptions, status?.config])

  const busy =
    saveConfig.isPending ||
    saveCredentials.isPending ||
    healthcheck.isPending ||
    activate.isPending ||
    disconnect.isPending

  const saveInputs = async () => {
    await saveConfig.mutateAsync({
      name: OPENAI_PROVIDER,
      config: {
        model: resolveModelID(modelInput, modelOptions, defaultModel),
        base_url: baseURL.trim() || DEFAULT_BASE_URL,
      },
    })
    if (apiKey.trim()) {
      await saveCredentials.mutateAsync({ name: OPENAI_PROVIDER, apiKey: apiKey.trim() })
      setAPIKey('')
    }
  }

  const handleSave = async () => {
    setError('')
    setMessage('')
    setActiveAction('save')
    try {
      await saveInputs()
      await activate.mutateAsync(OPENAI_PROVIDER)
      setMessage(t('settings.ai.saved'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settings.ai.saveFailed'))
    } finally {
      setActiveAction(null)
    }
  }

  const handleHealthcheck = async () => {
    setError('')
    setMessage('')
    setActiveAction('test')
    try {
      await saveInputs()
      const result = await healthcheck.mutateAsync(OPENAI_PROVIDER)
      setMessage(result.message || t('settings.ai.healthcheckPassed'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settings.ai.healthcheckFailed'))
    } finally {
      setActiveAction(null)
    }
  }

  const handleDisconnect = async () => {
    setError('')
    setMessage('')
    try {
      await disconnect.mutateAsync(OPENAI_PROVIDER)
      setConfirmDisconnect(false)
      setMessage(t('settings.ai.disconnected'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('settings.ai.disconnectFailed'))
    }
  }

  if (providersLoading || statusLoading) {
    return <p className="text-xs text-muted-foreground">{t('settings.ai.loading')}</p>
  }

  if (!provider) {
    return <p className="text-sm text-destructive">{t('settings.ai.providerMissing')}</p>
  }

  const credentialsStored = status?.credentials_stored ?? false
  const ready = status?.ready ?? false
  const requiresAPIKey = !credentialsStored && !apiKey.trim()

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h2 className="text-sm font-medium text-foreground">{provider.display_name}</h2>
          <p className="mt-1 max-w-3xl text-xs leading-relaxed text-muted-foreground">
            {t('settings.ai.description')}
          </p>
          <a
            href={OPENAI_API_DOCS_URL}
            target="_blank"
            rel="noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
          >
            {t('settings.ai.apiDocs')}
            <ExternalLink aria-hidden="true" size={12} />
          </a>
          <p className="mt-1 max-w-3xl text-xs leading-relaxed text-muted-foreground">
            {t('settings.ai.compatibleDescription')}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <StatusPill ready={ready} />
          {credentialsStored && (
            <button
              type="button"
              onClick={() => setConfirmDisconnect(true)}
              disabled={busy}
              aria-label={t('settings.ai.disconnect')}
              {...disconnectTipHandlers(t('settings.ai.disconnect'))}
              className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors hover:border-destructive hover:text-destructive disabled:cursor-not-allowed disabled:opacity-40"
            >
              <Unplug size={16} aria-hidden="true" />
            </button>
          )}
        </div>
      </div>

      <div className="grid max-w-3xl gap-5">
        <div>
          <span className="block text-sm text-muted-foreground">{t('settings.ai.baseUrl')}</span>
          <div className="mt-1 flex items-center rounded-md border border-border bg-input focus-within:border-primary focus-within:ring-1 focus-within:ring-ring">
            {baseURLEditing ? (
              <input
                ref={baseURLRef}
                aria-label={t('settings.ai.baseUrl')}
                value={baseURL}
                onChange={(event) => {
                  setBaseURL(event.target.value)
                  setError('')
                  setMessage('')
                }}
                className="min-w-0 flex-1 bg-transparent px-3 py-2 font-mono text-sm text-foreground focus:outline-none"
              />
            ) : (
              <span className="min-w-0 flex-1 select-none truncate px-3 py-2 font-mono text-sm text-muted-foreground">
                {baseURL}
              </span>
            )}
            <button
              type="button"
              aria-label={t('settings.ai.editBaseUrl')}
              onClick={() => {
                setBaseURLEditing(true)
                window.setTimeout(() => baseURLRef.current?.focus(), 0)
              }}
              className="mr-1 inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground"
            >
              <Pencil size={14} aria-hidden="true" />
            </button>
          </div>
        </div>

        <label className="block text-sm text-muted-foreground">
          {t('settings.ai.apiKey')}
          <div className="mt-1 flex items-center gap-2 rounded-md border border-border bg-input px-3 py-2 focus-within:border-primary focus-within:ring-1 focus-within:ring-ring">
            <KeyRound size={15} className="shrink-0 text-muted-foreground" />
            <input
              type="password"
              aria-label={t('settings.ai.apiKey')}
              value={apiKey}
              onChange={(event) => {
                setAPIKey(event.target.value)
                setError('')
                setMessage('')
              }}
              placeholder={
                credentialsStored ? '••••••••••••••••' : t('settings.ai.apiKeyPlaceholder')
              }
              className="min-w-0 flex-1 bg-transparent font-mono text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
            />
          </div>
          {!credentialsStored && (
            <span className="mt-1 block text-xs text-muted-foreground">
              {t('settings.ai.apiKeyHint')}
            </span>
          )}
        </label>

        <ModelCombobox value={modelInput} options={modelOptions} onChange={setModelInput} />
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={handleSave}
          disabled={busy || requiresAPIKey}
          className="rounded-md bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {activeAction === 'save' ? t('settings.ai.saving') : t('common.save')}
        </button>
        <button
          type="button"
          onClick={handleHealthcheck}
          disabled={busy || requiresAPIKey}
          className="rounded-md border border-border px-4 py-2 text-sm font-semibold text-foreground hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
        >
          {activeAction === 'test' ? t('settings.ai.testing') : t('settings.ai.test')}
        </button>
      </div>

      {message && <p className="text-xs text-green-500">{message}</p>}
      {error && <p className="text-xs text-destructive">{error}</p>}
      {disconnectTip}

      {confirmDisconnect && (
        <ConfirmModal
          title={t('settings.ai.disconnectTitle')}
          message={t('settings.ai.disconnectMessage')}
          confirmLabel={t('settings.ai.disconnect')}
          variant="destructive"
          onConfirm={handleDisconnect}
          onCancel={() => setConfirmDisconnect(false)}
        />
      )}
    </div>
  )
}
