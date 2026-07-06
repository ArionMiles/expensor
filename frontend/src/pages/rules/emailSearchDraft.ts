import type { ReaderMessageSample } from '@/api/types'

const draftPrefix = 'expensor.ruleEmailSearchDraft.'

export type RuleEmailSearchDraft = {
  subjectQuery: string
  messages: ReaderMessageSample[]
}

function draftStorageKey(id: string) {
  return `${draftPrefix}${id}`
}

export function saveRuleEmailSearchDraft(draft: RuleEmailSearchDraft) {
  const id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  sessionStorage.setItem(draftStorageKey(id), JSON.stringify(draft))
  return id
}

export function loadRuleEmailSearchDraft(id: string): RuleEmailSearchDraft | null {
  const raw = sessionStorage.getItem(draftStorageKey(id))
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as RuleEmailSearchDraft
    if (!Array.isArray(parsed.messages)) return null
    return parsed
  } catch {
    return null
  }
}
