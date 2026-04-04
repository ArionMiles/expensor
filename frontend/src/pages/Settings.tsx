import { cn } from '@/lib/utils'
import { useState } from 'react'
import { AppearanceSettings } from './settings/AppearanceSettings'
import { BucketsSettings } from './settings/BucketsSettings'
import { CategoriesSettings } from './settings/CategoriesSettings'
import { LabelsSettings } from './settings/LabelsSettings'
import { RulesSettings } from './settings/RulesSettings'

type SettingsTab = 'appearance' | 'categories' | 'buckets' | 'labels' | 'rules' | 'webhooks'

const TABS: { id: SettingsTab; label: string }[] = [
  { id: 'appearance', label: 'Appearance' },
  { id: 'categories', label: 'Categories' },
  { id: 'buckets', label: 'Buckets' },
  { id: 'labels', label: 'Labels' },
  { id: 'rules', label: 'Rules' },
  { id: 'webhooks', label: 'Webhooks' },
]

export default function Settings() {
  const [tab, setTab] = useState<SettingsTab>('appearance')

  return (
    <div className="mx-auto w-full max-w-4xl px-6 py-6">
      <h1 className="mb-6 text-lg font-semibold text-foreground">Settings</h1>
      <div className="mb-6 flex gap-1 border-b border-border">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              '-mb-px border-b-2 px-4 py-2 text-sm transition-colors',
              tab === t.id
                ? 'border-primary font-medium text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
      {tab === 'appearance' && <AppearanceSettings />}
      {tab === 'categories' && <CategoriesSettings />}
      {tab === 'buckets' && <BucketsSettings />}
      {tab === 'labels' && <LabelsSettings />}
      {tab === 'rules' && <RulesSettings />}
      {tab === 'webhooks' && <p className="text-sm text-muted-foreground">Coming soon.</p>}
    </div>
  )
}
