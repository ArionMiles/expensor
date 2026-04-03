import { getLabelColor } from '@/lib/utils'

interface LabelChipProps {
  label: string
  onRemove?: () => void
  className?: string
}

export function LabelChip({ label, onRemove, className }: LabelChipProps) {
  const color = getLabelColor(label)

  return (
    <span
      className={className}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '4px',
        padding: '2px 8px',
        fontSize: '11px',
        border: `1px solid ${color}40`,
        backgroundColor: `${color}18`,
        color: color,
        borderRadius: 'calc(var(--radius) - 2px)',
        whiteSpace: 'nowrap',
        lineHeight: '1.4',
      }}
    >
      {label}
      {onRemove && (
        <button
          onClick={onRemove}
          aria-label={`Remove label ${label}`}
          style={{
            background: 'none',
            border: 'none',
            color: 'inherit',
            cursor: 'pointer',
            padding: '0',
            lineHeight: 1,
            opacity: 0.6,
            fontSize: '13px',
          }}
          onMouseOver={(e) => {
            (e.currentTarget as HTMLButtonElement).style.opacity = '1'
          }}
          onMouseOut={(e) => {
            (e.currentTarget as HTMLButtonElement).style.opacity = '0.6'
          }}
        >
          ×
        </button>
      )}
    </span>
  )
}
