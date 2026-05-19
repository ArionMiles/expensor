import { useState } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

type Placement = 'below' | 'right'
type XAlign = 'start' | 'center' | 'end'

/**
 * Portal tooltip hook — escapes any overflow:hidden ancestor.
 *
 * Usage:
 *   const { tip, handlers } = useTooltip()
 *   <button {...handlers('Save')}>…</button>
 *   {tip}
 *
 * placement 'below' (default): tooltip centred below the element.
 * placement 'right':            tooltip centred to the right (used by sidebar).
 */
export function useTooltip(placement: Placement = 'below') {
  const [state, setState] = useState<{
    label: string
    x: number
    y: number
    xAlign: XAlign
  } | null>(null)

  const handlers = (label: string) => ({
    onMouseEnter: (e: React.MouseEvent<Element>) => {
      const r = e.currentTarget.getBoundingClientRect()
      if (placement === 'right') {
        setState({ label, x: r.right + 8, y: r.top + r.height / 2, xAlign: 'start' })
        return
      }

      const edgePadding = 160
      if (window.innerWidth - r.right < edgePadding) {
        setState({ label, x: r.right, y: r.bottom + 6, xAlign: 'end' })
        return
      }
      if (r.left < edgePadding) {
        setState({ label, x: r.left, y: r.bottom + 6, xAlign: 'start' })
        return
      }
      setState({ label, x: r.left + r.width / 2, y: r.bottom + 6, xAlign: 'center' })
    },
    onMouseLeave: () => setState(null),
  })

  const tip =
    state &&
    createPortal(
      <div
        className={cn(
          'pointer-events-none fixed z-50 max-w-[calc(100vw-1rem)] whitespace-normal rounded bg-foreground px-2 py-1 text-xs text-background shadow',
          placement === 'right'
            ? '-translate-y-1/2'
            : state.xAlign === 'center'
              ? '-translate-x-1/2'
              : state.xAlign === 'end'
                ? '-translate-x-full'
                : '',
        )}
        style={{ left: state.x, top: state.y }}
      >
        {state.label}
      </div>,
      document.body,
    )

  return { handlers, tip }
}
