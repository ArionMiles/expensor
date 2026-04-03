import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import './index.css'

// Apply persisted theme immediately to avoid flash
const stored = localStorage.getItem('theme')
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
document.documentElement.setAttribute(
  'data-theme',
  stored === 'light' ? 'light' : stored === 'dark' ? 'dark' : prefersDark ? 'dark' : 'light',
)

const root = document.getElementById('root')
if (!root) throw new Error('Root element not found')

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
