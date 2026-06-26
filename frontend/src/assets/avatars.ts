import catalogData from '@avatar-content/catalog.json'
import defaultSvg from '@avatar-content/default.svg?raw'
import ledgerSvg from '@avatar-content/ledger.svg?raw'
import walletSvg from '@avatar-content/wallet.svg?raw'

export type AvatarKey = 'default' | 'ledger' | 'wallet'

export type AvatarCatalogEntry = {
  key: AvatarKey
  label: string
}

export const avatarCatalog = catalogData as readonly AvatarCatalogEntry[]
export const avatarKeys = avatarCatalog.map((avatar) => avatar.key)

export const avatarByKey: Record<AvatarKey, string> = {
  default: defaultSvg,
  ledger: ledgerSvg,
  wallet: walletSvg,
}

export function isAvatarKey(key: string): key is AvatarKey {
  return Object.prototype.hasOwnProperty.call(avatarByKey, key)
}
