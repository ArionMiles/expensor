import { getLabelColor } from '@/lib/utils'

interface LabelChipProps {
  label: string
  color?: string
  onRemove?: () => void
  className?: string
}

export function LabelChip({ label, color, onRemove, className }: LabelChipProps) {
  const resolvedColor = color ?? getLabelColor(label)

  return (
    <span
      className={className}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '4px',
        padding: '2px 8px',
        fontSize: '11px',
        border: `1px solid ${resolvedColor}40`,
        backgroundColor: `${resolvedColor}18`,
        color: resolvedColor,
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
            ;(e.currentTarget as HTMLButtonElement).style.opacity = '1'
          }}
          onMouseOut={(e) => {
            ;(e.currentTarget as HTMLButtonElement).style.opacity = '0.6'
          }}
        >
          ×
        </button>
      )}
    </span>
  )
}
