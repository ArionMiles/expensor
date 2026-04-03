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
        <h2 className="text-base font-semibold text-foreground mb-1">Upload credentials</h2>
        <p className="text-sm text-muted-foreground">
          Upload your{' '}
          <code className="font-mono text-primary text-xs">client_secret.json</code> from
          the Google Cloud Console.
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
          'border-2 border-dashed rounded-lg p-8 text-center transition-colors',
          dragOver
            ? 'border-primary bg-primary/10'
            : 'border-border bg-secondary/30 hover:bg-secondary/50',
        )}
      >
        {file ? (
          <div className="space-y-2">
            <p className="text-sm text-success">✓ {file.name}</p>
            <p className="text-xs text-muted-foreground">{(file.size / 1024).toFixed(1)} KB</p>
            <button
              onClick={() => setFile(null)}
              className="text-xs text-muted-foreground hover:text-foreground underline"
            >
              Remove
            </button>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">Drag & drop your JSON file here</p>
            <p className="text-xs text-muted-foreground">or</p>
            <label className="inline-block cursor-pointer">
              <span className="px-3 py-1.5 text-xs rounded-md border border-border bg-card text-foreground hover:bg-accent transition-colors">
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

      {isSuccess && (
        <p className="text-xs text-success">✓ Credentials uploaded successfully</p>
      )}

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          ← Back
        </button>
        <button
          onClick={handleUpload}
          disabled={!file || isPending}
          className={cn(
            'px-4 py-2 text-sm rounded-md transition-colors',
            file && !isPending
              ? 'bg-primary text-primary-foreground hover:bg-primary/90'
              : 'bg-secondary text-muted-foreground cursor-not-allowed opacity-50',
          )}
        >
          {isPending ? 'Uploading...' : 'Upload & continue →'}
        </button>
      </div>
    </div>
  )
}
