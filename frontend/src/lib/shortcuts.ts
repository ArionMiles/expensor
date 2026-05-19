function currentPlatform() {
  return typeof navigator === 'undefined' ? '' : navigator.platform
}

export function shortcutModifier(platform = currentPlatform()) {
  return /Mac|iPhone|iPad|iPod/.test(platform) ? '⌘' : 'Ctrl'
}

export function shortcutLabel(key: string, platform?: string) {
  return `${shortcutModifier(platform)} + ${key}`
}
