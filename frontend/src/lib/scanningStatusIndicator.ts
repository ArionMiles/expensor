const STORAGE_KEY = 'expensor.scanningStatusBreathing'
const CHANGE_EVENT = 'expensor:scanningStatusBreathingChanged'

export function isScanningStatusBreathingEnabled() {
  if (typeof window === 'undefined') return true
  try {
    return window.localStorage?.getItem(STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

export function setScanningStatusBreathingEnabled(enabled: boolean) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage?.setItem(STORAGE_KEY, String(enabled))
  } catch {
    // keep the in-memory UI update even when storage is unavailable
  }
  window.dispatchEvent(new CustomEvent(CHANGE_EVENT, { detail: { enabled } }))
}

export function toggleScanningStatusBreathing() {
  const next = !isScanningStatusBreathingEnabled()
  setScanningStatusBreathingEnabled(next)
  return next
}

export function subscribeScanningStatusBreathing(listener: (enabled: boolean) => void) {
  if (typeof window === 'undefined') return () => {}

  const notify = () => listener(isScanningStatusBreathingEnabled())
  const onStorage = (event: StorageEvent) => {
    if (event.key === STORAGE_KEY) notify()
  }

  window.addEventListener(CHANGE_EVENT, notify)
  window.addEventListener('storage', onStorage)
  return () => {
    window.removeEventListener(CHANGE_EVENT, notify)
    window.removeEventListener('storage', onStorage)
  }
}
