import { type KeyboardEvent, useEffect, useId, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  useActivateLLMProvider,
  useDisconnectLLMProvider,
  useHealthCheckLLMProvider,
  useLLMProviderStatuses,
  useLLMProviders,
  useSaveLLMProviderConfig,
  useSaveLLMProviderCredentials,
} from '@/api/queries'
import type { LLMProviderInfo, LLMProviderModelOption, LLMProviderStatus } from '@/api/types'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useTooltip } from '@/hooks/useTooltip'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'
import {
  AlertCircle,
  CheckCircle2,
  CircleDot,
  ExternalLink,
  KeyRound,
  Pencil,
  PlugZap,
  Save,
  ShieldCheck,
  Unplug,
} from 'lucide-react'

const PREFERRED_PROVIDER = 'openai'

function StatusPill({ status }: { status?: LLMProviderStatus }) {
  const { t } = useI18n()
  const active = status?.active ?? false
  const configured = status?.credentials_stored ?? false

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium',
        active && 'border-green-500/40 text-green-500',
        !active && configured && 'border-primary/40 text-primary',
        !active && !configured && 'border-warning/40 text-warning',
      )}
    >
      {active ? (
        <CheckCircle2 size={13} />
      ) : configured ? (
        <CircleDot size={13} />
      ) : (
        <AlertCircle size={13} />
      )}
      {active
        ? t('settings.ai.statusActive')
        : configured
          ? t('settings.ai.statusConfigured')
          : t('settings.ai.statusNeedsSetup')}
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

function schemaDefault(provider: LLMProviderInfo, field: string) {
  const properties = provider.config_schema?.properties
  if (!properties || typeof properties !== 'object' || Array.isArray(properties)) return ''
  const property = (properties as Record<string, unknown>)[field]
  if (!property || typeof property !== 'object' || Array.isArray(property)) return ''
  const value = (property as Record<string, unknown>).default
  return typeof value === 'string' ? value : ''
}

function ModelCombobox({
  provider,
  value,
  options,
  onChange,
}: {
  provider: LLMProviderInfo
  value: string
  options: LLMProviderModelOption[]
  onChange: (value: string) => void
}) {
  const { t } = useI18n()
  const inputID = useId()
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
      <label className="block text-sm text-muted-foreground" htmlFor={inputID}>
        {t('settings.ai.model')}
      </label>
      <div className="relative mt-1">
        <input
          ref={inputRef}
          id={inputID}
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
        label={t('settings.ai.modelOptions', { provider: provider.display_name })}
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

function DataUseNotice({ provider }: { provider: LLMProviderInfo }) {
  const { t } = useI18n()
  const detail =
    provider.data_use.mode === 'free_tier_improvement'
      ? t('settings.ai.dataUseFreeTier')
      : t('settings.ai.dataUseNoTraining')

  return (
    <div className="border-l-2 border-warning/50 pl-3">
      <div className="flex items-center gap-2 text-sm font-medium text-foreground">
        <ShieldCheck size={15} aria-hidden="true" className="text-warning" />
        {t('settings.ai.dataUseTitle')}
      </div>
      <p className="mt-1 max-w-3xl text-xs leading-relaxed text-muted-foreground">{detail}</p>
      <a
        href={provider.data_use.policy_url}
        target="_blank"
        rel="noreferrer"
        className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
      >
        {t('settings.ai.dataPolicy', { provider: provider.display_name })}
        <ExternalLink aria-hidden="true" size={12} />
      </a>
    </div>
  )
}

function ProviderSettingsForm({
  provider,
  status,
}: {
  provider: LLMProviderInfo
  status?: LLMProviderStatus
}) {
  const { t } = useI18n()
  const { handlers: actionTipHandlers, tip: actionTip } = useTooltip()
  const saveConfig = useSaveLLMProviderConfig()
  const saveCredentials = useSaveLLMProviderCredentials()
  const healthcheck = useHealthCheckLLMProvider()
  const activate = useActivateLLMProvider()
  const disconnect = useDisconnectLLMProvider()

  const modelOptions = useMemo(() => provider.model_options ?? [], [provider.model_options])
  const schemaModel = schemaDefault(provider, 'model')
  const defaultModel =
    modelOptions.find((option) => option.recommended)?.id || modelOptions[0]?.id || schemaModel
  const defaultBaseURL = schemaDefault(provider, 'base_url')
  const supportsCustomBaseURL = defaultBaseURL !== ''

  const [modelInput, setModelInput] = useState(displayModelValue(defaultModel, modelOptions))
  const [baseURL, setBaseURL] = useState(defaultBaseURL)
  const [baseURLEditing, setBaseURLEditing] = useState(false)
  const [apiKey, setAPIKey] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [confirmDisconnect, setConfirmDisconnect] = useState(false)
  const [activeAction, setActiveAction] = useState<'save' | 'test' | null>(null)
  const baseURLRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setModelInput(displayModelValue(status?.config.model || defaultModel, modelOptions))
    setBaseURL(status?.config.base_url || defaultBaseURL)
  }, [defaultBaseURL, defaultModel, modelOptions, status?.config])

  const busy =
    saveConfig.isPending ||
    saveCredentials.isPending ||
    healthcheck.isPending ||
    activate.isPending ||
    disconnect.isPending

  const saveInputs = async () => {
    const config = {
      model: resolveModelID(modelInput, modelOptions, defaultModel),
      ...(supportsCustomBaseURL ? { base_url: baseURL.trim() || defaultBaseURL } : {}),
    }
    await saveConfig.mutateAsync({
      name: provider.name,
      config,
    })
    if (apiKey.trim()) {
      await saveCredentials.mutateAsync({ name: provider.name, apiKey: apiKey.trim() })
      setAPIKey('')
    }
  }

  const handleSave = async () => {
    setError('')
    setMessage('')
    setActiveAction('save')
    try {
      await saveInputs()
      await activate.mutateAsync(provider.name)
      setMessage(t('settings.ai.saved', { provider: provider.display_name }))
    } catch (caught) {
      setError(
        caught instanceof Error
          ? caught.message
          : t('settings.ai.saveFailed', { provider: provider.display_name }),
      )
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
      const result = await healthcheck.mutateAsync(provider.name)
      setMessage(
        result.message || t('settings.ai.healthcheckPassed', { provider: provider.display_name }),
      )
    } catch (caught) {
      setError(
        caught instanceof Error
          ? caught.message
          : t('settings.ai.healthcheckFailed', { provider: provider.display_name }),
      )
    } finally {
      setActiveAction(null)
    }
  }

  const handleDisconnect = async () => {
    setError('')
    setMessage('')
    try {
      await disconnect.mutateAsync(provider.name)
      setConfirmDisconnect(false)
      setMessage(t('settings.ai.disconnected', { provider: provider.display_name }))
    } catch (caught) {
      setError(
        caught instanceof Error
          ? caught.message
          : t('settings.ai.disconnectFailed', { provider: provider.display_name }),
      )
    }
  }

  const credentialsStored = status?.credentials_stored ?? false
  const requiresAPIKey = provider.auth_type === 'api_key' && !credentialsStored && !apiKey.trim()
  const [apiKeyHelpPrefix, apiKeyHelpSuffix] = t('settings.ai.apiKeyHelp').split('{dashboard}')
  const saveLabel = activeAction === 'save' ? t('settings.ai.saving') : t('settings.ai.saveAndUse')
  const testLabel = activeAction === 'test' ? t('settings.ai.testing') : t('settings.ai.test')

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          {provider.api_key_url && provider.api_key_link_text && (
            <p className="text-xs text-muted-foreground">
              {apiKeyHelpPrefix}
              <a
                href={provider.api_key_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
              >
                {provider.api_key_link_text}
                <ExternalLink aria-hidden="true" size={12} />
              </a>
              {apiKeyHelpSuffix}
            </p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <StatusPill status={status} />
        </div>
      </div>

      <div className="grid max-w-3xl gap-5">
        {supportsCustomBaseURL && (
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
        )}

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
          <span className="mt-1 block text-xs text-muted-foreground">
            {credentialsStored ? t('settings.ai.apiKeyStoredHint') : t('settings.ai.apiKeyHint')}
          </span>
        </label>

        <ModelCombobox
          provider={provider}
          value={modelInput}
          options={modelOptions}
          onChange={setModelInput}
        />
      </div>

      <DataUseNotice provider={provider} />

      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex" {...actionTipHandlers(saveLabel)}>
          <button
            type="button"
            onClick={handleSave}
            disabled={busy || requiresAPIKey}
            aria-label={saveLabel}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Save size={16} aria-hidden="true" />
          </button>
        </span>
        <span className="inline-flex" {...actionTipHandlers(testLabel)}>
          <button
            type="button"
            onClick={handleHealthcheck}
            disabled={busy || requiresAPIKey}
            aria-label={testLabel}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-foreground hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
          >
            <PlugZap size={16} aria-hidden="true" />
          </button>
        </span>
        {credentialsStored && (
          <span className="inline-flex" {...actionTipHandlers(t('settings.ai.disconnect'))}>
            <button
              type="button"
              onClick={() => setConfirmDisconnect(true)}
              disabled={busy}
              aria-label={t('settings.ai.disconnect')}
              className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors hover:border-destructive hover:bg-destructive/5 hover:text-destructive disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Unplug size={16} aria-hidden="true" />
            </button>
          </span>
        )}
      </div>

      {message && <p className="text-xs text-green-500">{message}</p>}
      {error && <p className="text-xs text-destructive">{error}</p>}
      {actionTip}

      {confirmDisconnect && (
        <ConfirmModal
          title={t('settings.ai.disconnectTitle', { provider: provider.display_name })}
          message={t('settings.ai.disconnectMessage', { provider: provider.display_name })}
          confirmLabel={t('settings.ai.disconnect')}
          variant="destructive"
          onConfirm={handleDisconnect}
          onCancel={() => setConfirmDisconnect(false)}
        />
      )}
    </div>
  )
}

export function AISettings() {
  const { t } = useI18n()
  const [searchParams, setSearchParams] = useSearchParams()
  const { data: providers = [], isLoading: providersLoading } = useLLMProviders()
  const providerNames = useMemo(() => providers.map((provider) => provider.name), [providers])
  const statusQueries = useLLMProviderStatuses(providerNames)
  const statuses = statusQueries
    .map((query) => query.data)
    .filter((status): status is LLMProviderStatus => Boolean(status))
  const statusesLoading = statusQueries.some((query) => query.isLoading)
  const requestedProvider = searchParams.get('provider')
  const activeProvider = statuses.find((status) => status.active)?.name
  const selectedProviderName =
    providers.find((provider) => provider.name === requestedProvider)?.name ||
    activeProvider ||
    providers.find((provider) => provider.name === PREFERRED_PROVIDER)?.name ||
    providers[0]?.name ||
    ''
  const selectedProvider = providers.find((provider) => provider.name === selectedProviderName)
  const selectedStatus = statuses.find((status) => status.name === selectedProviderName)

  const selectProvider = (name: string) => {
    const next = new URLSearchParams(searchParams)
    next.set('provider', name)
    setSearchParams(next, { replace: true })
  }

  const handleProviderTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let nextIndex: number | undefined
    if (event.key === 'ArrowLeft') nextIndex = (index - 1 + providers.length) % providers.length
    if (event.key === 'ArrowRight') nextIndex = (index + 1) % providers.length
    if (event.key === 'Home') nextIndex = 0
    if (event.key === 'End') nextIndex = providers.length - 1
    if (nextIndex === undefined) return

    event.preventDefault()
    const nextProvider = providers[nextIndex]
    selectProvider(nextProvider.name)
    window.setTimeout(
      () => document.getElementById(`llm-provider-tab-${nextProvider.name}`)?.focus(),
      0,
    )
  }

  useEffect(() => {
    if (providersLoading || statusesLoading || !selectedProviderName) return
    if (requestedProvider === selectedProviderName) return
    const next = new URLSearchParams(searchParams)
    next.set('provider', selectedProviderName)
    setSearchParams(next, { replace: true })
  }, [
    providersLoading,
    requestedProvider,
    searchParams,
    selectedProviderName,
    setSearchParams,
    statusesLoading,
  ])

  if (providersLoading || statusesLoading) {
    return <p className="text-xs text-muted-foreground">{t('settings.ai.loading')}</p>
  }

  if (!selectedProvider) {
    return <p className="text-sm text-destructive">{t('settings.ai.providerMissing')}</p>
  }

  return (
    <div className="space-y-8">
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {t('settings.ai.providerLabel')}
        </p>
        <div
          className="mt-2 flex flex-wrap gap-1 border-b border-border"
          role="tablist"
          aria-label={t('settings.ai.providerLabel')}
        >
          {providers.map((provider, index) => {
            const status = statuses.find((candidate) => candidate.name === provider.name)
            const selected = provider.name === selectedProviderName
            return (
              <button
                key={provider.name}
                id={`llm-provider-tab-${provider.name}`}
                type="button"
                role="tab"
                aria-selected={selected}
                aria-controls={`llm-provider-panel-${provider.name}`}
                tabIndex={selected ? 0 : -1}
                onClick={() => selectProvider(provider.name)}
                onKeyDown={(event) => handleProviderTabKeyDown(event, index)}
                className={cn(
                  '-mb-px inline-flex items-center gap-2 border-b-2 px-3 py-2 text-sm transition-colors',
                  selected
                    ? 'border-primary font-medium text-foreground'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
                )}
              >
                <span
                  className={cn(
                    'h-1.5 w-1.5 rounded-full',
                    status?.active
                      ? 'bg-green-500'
                      : status?.credentials_stored
                        ? 'bg-primary'
                        : 'bg-muted-foreground/50',
                  )}
                />
                {provider.display_name}
              </button>
            )
          })}
        </div>
      </div>

      <div
        id={`llm-provider-panel-${selectedProvider.name}`}
        role="tabpanel"
        aria-labelledby={`llm-provider-tab-${selectedProvider.name}`}
      >
        <ProviderSettingsForm
          key={selectedProvider.name}
          provider={selectedProvider}
          status={selectedStatus}
        />
      </div>
    </div>
  )
}
