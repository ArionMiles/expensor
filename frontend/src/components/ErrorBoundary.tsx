import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="rounded-lg border border-destructive bg-card p-6">
          <p className="mb-1 text-sm font-medium text-destructive">Something went wrong</p>
          <p className="text-xs text-muted-foreground">
            {this.state.error?.message ?? 'Unknown error'}
          </p>
          <button
            onClick={() => this.setState({ hasError: false, error: null })}
            className="mt-3 rounded-md border border-border px-3 py-1.5 text-xs text-foreground transition-colors hover:bg-accent"
          >
            Try again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
