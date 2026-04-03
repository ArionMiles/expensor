import { intensityColor } from '@/lib/heatmap'

/**
 * Renders five colour squares illustrating the intensity scale used by heatmap
 * components: empty (muted) → low → medium → high → very high (primary).
 */
export function HeatmapLegend() {
  return (
    <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span>Less</span>
      {[0, 1, 2, 3, 4].map((step) => (
        <svg key={step} width={12} height={12} aria-hidden="true">
          <rect
            x={0}
            y={0}
            width={12}
            height={12}
            rx={2}
            fill={intensityColor(step, 4)}
          />
        </svg>
      ))}
      <span>More</span>
    </div>
  )
}
