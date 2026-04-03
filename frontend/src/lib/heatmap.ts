/**
 * Maps a value onto a 5-step colour scale from muted (zero) to primary (max).
 * Steps 1–4 use increasing opacity of the CSS --primary variable.
 *
 * Uses the modern CSS `hsl(H S% L% / alpha)` syntax, which works because
 * --primary is stored as raw HSL components (e.g. "217.2 91.2% 59.8%").
 */
export function intensityColor(value: number, max: number): string {
  if (max === 0 || value === 0) return 'hsl(var(--border))'
  const ratio = Math.min(value / max, 1)
  const step = Math.ceil(ratio * 4) // 1–4
  const opacities = [0.2, 0.4, 0.65, 0.9]
  return `hsl(var(--primary) / ${opacities[step - 1]})`
}
