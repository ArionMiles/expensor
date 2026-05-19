import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { ThemeProvider } from './components/ThemeProvider'
import './index.css'

const E2E_MOCKS_KEY = 'expensor:e2e-mocks'

// Apply persisted theme class immediately to avoid flash
;(() => {
  try {
    const stored = localStorage.getItem('theme')
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    const resolved =
      stored === 'light' ? 'light' : stored === 'dark' ? 'dark' : prefersDark ? 'dark' : 'light'
    document.documentElement.classList.add(resolved)
  } catch {
    // ignore
  }
})()

async function maybeEnableE2EMocks() {
  try {
    if (window.localStorage.getItem(E2E_MOCKS_KEY) !== '1') {
      return
    }

    const { worker } = await import('./mocks/browser')
    await worker.start({
      onUnhandledRequest: 'error',
      serviceWorker: {
        url: '/mockServiceWorker.js',
      },
    })
  } catch (error) {
    console.error('Failed to start browser mocks', error)
    throw error
  }
}

async function bootstrap() {
  await maybeEnableE2EMocks()

  const root = document.getElementById('root')
  if (!root) throw new Error('Root element not found')

  createRoot(root).render(
    <StrictMode>
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </StrictMode>,
  )
}

void bootstrap()
