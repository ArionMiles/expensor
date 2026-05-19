import '@testing-library/jest-dom/vitest'
import 'vitest-axe/extend-expect'
import { afterAll, afterEach, beforeAll, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import { server } from './server'

Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
})

window.HTMLElement.prototype.scrollIntoView = vi.fn()

beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' })
})

afterEach(() => {
  cleanup()
  server.resetHandlers()
  window.localStorage?.clear?.()
  vi.restoreAllMocks()
})

afterAll(() => {
  server.close()
})
