import { useUploadCredentials } from '@/api/queries'
import { cn } from '@/lib/utils'
import { useCallback, useState } from 'react'

interface UploadCredentialsProps {
  readerName: string
  onNext: () => void
  onBack: () => void
}

export function UploadCredentials({ readerName, onNext, onBack }: UploadCredentialsProps) {
  const [dragOver, setDragOver] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const { mutate: upload, isPending, error, isSuccess } = useUploadCredentials()

  const handleFile = useCallback((f: File) => {
    if (!f.name.endsWith('.json')) return
    setFile(f)
  }, [])

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      const dropped = e.dataTransfer.files[0]
      if (dropped) handleFile(dropped)
    },
    [handleFile],
  )

  const handleFileInput = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const picked = e.target.files?.[0]
      if (picked) handleFile(picked)
    },
    [handleFile],
  )

  const handleUpload = () => {
    if (!file) return
    upload({ readerName, file }, { onSuccess: () => onNext() })
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="mb-1 text-base font-semibold text-foreground">Upload credentials</h2>
        <p className="text-sm text-muted-foreground">
          Upload your <code className="font-mono text-xs text-primary">client_secret.json</code>{' '}
          from the Google Cloud Console.
        </p>
      </div>

      <div
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
        className={cn(
          'rounded-lg border-2 border-dashed p-8 text-center transition-colors',
          dragOver
            ? 'border-primary bg-primary/10'
            : 'border-border bg-secondary/30 hover:bg-secondary/50',
        )}
      >
        {file ? (
          <div className="space-y-2">
            <div className="flex min-w-0 items-center justify-center gap-1.5">
              <span className="flex-shrink-0 text-sm text-success">✓</span>
              <span className="truncate font-mono text-sm text-success" title={file.name}>
                {file.name}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">{(file.size / 1024).toFixed(1)} KB</p>
            <button
              onClick={() => setFile(null)}
              className="text-xs text-muted-foreground underline hover:text-foreground"
            >
              Remove
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">Drag & drop your JSON file here</p>
            <p className="text-xs text-muted-foreground">or</p>
            <label className="inline-block cursor-pointer">
              <span className="rounded-md border border-border bg-card px-3 py-1.5 text-xs text-foreground transition-colors hover:bg-accent">
                Browse files
              </span>
              <input
                type="file"
                accept=".json,application/json"
                onChange={handleFileInput}
                className="sr-only"
                aria-label="Upload client_secret.json"
              />
            </label>
          </div>
        )}
      </div>

      {error && (
        <p className="text-xs text-destructive" role="alert">
          {error instanceof Error ? error.message : 'Upload failed'}
        </p>
      )}

      {isSuccess && <p className="text-xs text-success">✓ Credentials uploaded successfully</p>}

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          ← Back
        </button>
        <button
          onClick={handleUpload}
          disabled={!file || isPending}
          className={cn(
            'rounded-md px-4 py-2 text-sm transition-colors',
            file && !isPending
              ? 'bg-primary text-primary-foreground hover:bg-primary/90'
              : 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50',
          )}
        >
          {isPending ? 'Uploading...' : 'Upload & continue →'}
        </button>
      </div>
    </div>
  )
}
