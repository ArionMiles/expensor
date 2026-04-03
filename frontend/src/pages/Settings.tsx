import { useState } from 'react'
import { cn } from '@/lib/utils'
import { AppearanceSettings } from './settings/AppearanceSettings'

type SettingsTab = 'appearance' | 'categories' | 'buckets' | 'labels' | 'webhooks'

const TABS: { id: SettingsTab; label: string }[] = [
  { id: 'appearance', label: 'Appearance' },
  { id: 'categories', label: 'Categories' },
  { id: 'buckets', label: 'Buckets' },
  { id: 'labels', label: 'Labels' },
  { id: 'webhooks', label: 'Webhooks' },
]

const COMING_SOON_TABS: SettingsTab[] = ['categories', 'buckets', 'labels', 'webhooks']

export default function Settings() {
  const [tab, setTab] = useState<SettingsTab>('appearance')

  return (
    <div className="px-6 py-6 max-w-4xl mx-auto w-full">
      <h1 className="text-lg font-semibold text-foreground mb-6">Settings</h1>

      {/* Tab bar */}
      <div className="flex gap-1 border-b border-border mb-6">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              'px-4 py-2 text-sm transition-colors border-b-2 -mb-px',
              tab === t.id
                ? 'border-primary text-foreground font-medium'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {tab === 'appearance' && <AppearanceSettings />}
      {COMING_SOON_TABS.includes(tab) && (
        <p className="text-sm text-muted-foreground">Coming soon.</p>
      )}
    </div>
  )
}
