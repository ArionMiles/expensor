import { describe, expect, it } from 'vitest'
import { normalizeTimezone } from './timezone'

describe('normalizeTimezone', () => {
  it('maps browser aliases onto the supported timezone name', () => {
    expect(normalizeTimezone('Asia/Kolkata')).toBe('Asia/Calcutta')
  })

  it('preserves already-supported timezone names', () => {
    expect(normalizeTimezone('Asia/Calcutta')).toBe('Asia/Calcutta')
  })

  it('returns an empty string for blank values', () => {
    expect(normalizeTimezone('   ')).toBe('')
  })
})
