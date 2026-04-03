import { useSaveReaderConfig } from '@/api/queries'
import type { ConfigField } from '@/api/types'
import { cn } from '@/lib/utils'
import { useState } from 'react'

interface ConfigureStepProps {
  readerName: string
  configSchema: ConfigField[]
  onNext: () => void
  onBack: () => void
}

export function ConfigureStep({
  readerName,
  configSchema,
  onNext,
  onBack,
}: ConfigureStepProps) {
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
        setValidationError(`"${field.label}" is required`)
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
          <h2 className="text-base font-semibold text-foreground mb-1">Configure reader</h2>
          <p className="text-sm text-muted-foreground">
            No configuration required for this reader.
          </p>
        </div>
        <div className="flex items-center justify-between">
          <button
            onClick={onBack}
            className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            ← Back
          </button>
          <button
            onClick={onNext}
            className="px-4 py-2 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            Next →
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-base font-semibold text-foreground mb-1">Configure reader</h2>
        <p className="text-sm text-muted-foreground">
          Set the required options for the{' '}
          <span className="font-mono text-primary">{readerName}</span> reader.
        </p>
      </div>

      <div className="space-y-4">
        {configSchema.map((field) => (
          <div key={field.name} className="space-y-1.5">
            <label
              htmlFor={`config-${field.name}`}
              className="block text-xs font-medium text-muted-foreground uppercase tracking-wider"
            >
              {field.label}
              {field.required && <span className="text-destructive ml-1">*</span>}
            </label>
            {field.type === 'textarea' ? (
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
          </div>
        ))}
      </div>

      {(validationError || error) && (
        <p className="text-xs text-destructive" role="alert">
          {validationError ?? (error instanceof Error ? error.message : 'Save failed')}
        </p>
      )}

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          ← Back
        </button>
        <button
          onClick={handleSubmit}
          disabled={isPending}
          className={cn(
            'px-4 py-2 text-sm rounded-md transition-colors',
            isPending
              ? 'bg-secondary text-muted-foreground cursor-not-allowed opacity-50'
              : 'bg-primary text-primary-foreground hover:bg-primary/90',
          )}
        >
          {isPending ? 'Saving...' : 'Save & continue →'}
        </button>
      </div>
    </div>
  )
}
