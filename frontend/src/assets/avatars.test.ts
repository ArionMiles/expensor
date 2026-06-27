import { describe, expect, it } from 'vitest'

import { avatarByKey, avatarCatalog, avatarKeys, isAvatarKey } from './avatars'

describe('avatar assets', () => {
  it('exposes the bundled account avatar catalog in key order', () => {
    expect(avatarKeys).toEqual(['default', 'ledger', 'wallet'])
    expect(avatarCatalog.map((avatar) => avatar.key)).toEqual(avatarKeys)
    expect(avatarCatalog.map((avatar) => avatar.label)).toEqual(['Default', 'Ledger', 'Wallet'])
  })

  it('maps each catalog key to a safe inline SVG asset', () => {
    for (const key of avatarKeys) {
      const svg = avatarByKey[key]

      expect(svg.trim().startsWith('<svg')).toBe(true)
      expect(svg).toContain('viewBox=')
      expect(svg).not.toMatch(/<script|<foreignObject|<image/i)
      expect(svg).not.toMatch(/\son[a-z]+\s*=/i)
      expect(svg).not.toMatch(/\s(?:href|src)=["'](?:https?:|data:|javascript:)/i)
    }
  })

  it('checks keys against the catalog', () => {
    expect(isAvatarKey('default')).toBe(true)
    expect(isAvatarKey('unknown')).toBe(false)
  })
})
