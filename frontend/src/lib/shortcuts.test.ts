import { describe, expect, it } from 'vitest'
import { shortcutLabel, shortcutModifier } from './shortcuts'

describe('shortcuts', () => {
  it('uses the command symbol on Apple platforms', () => {
    expect(shortcutModifier('MacIntel')).toBe('⌘')
    expect(shortcutLabel('K', 'iPhone')).toBe('⌘ + K')
  })

  it('uses Ctrl on non-Apple platforms', () => {
    expect(shortcutModifier('Win32')).toBe('Ctrl')
    expect(shortcutLabel('.', 'Linux x86_64')).toBe('Ctrl + .')
  })
})
