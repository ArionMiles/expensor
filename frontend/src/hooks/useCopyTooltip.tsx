import { useEffect, useRef, useState, type FocusEvent, type MouseEvent } from 'react'
import { createPortal } from 'react-dom'

export function useCopyTooltip(initialLabel: string) {
  const [tooltip, setTooltip] = useState<{ label: string; x: number; y: number } | null>(null)
  const labelRef = useRef(initialLabel)

  useEffect(() => {
    labelRef.current = initialLabel
    setTooltip((current) => (current ? { ...current, label: initialLabel } : current))
  }, [initialLabel])

  const handlers = {
    onMouseEnter: (event: MouseEvent<Element>) => {
      const rect = event.currentTarget.getBoundingClientRect()
      setTooltip({
        label: labelRef.current,
        x: rect.left + rect.width / 2,
        y: rect.bottom + 6,
      })
    },
    onMouseLeave: () => setTooltip(null),
    onFocus: (event: FocusEvent<Element>) => {
      const rect = event.currentTarget.getBoundingClientRect()
      setTooltip({
        label: labelRef.current,
        x: rect.left + rect.width / 2,
        y: rect.bottom + 6,
      })
    },
    onBlur: () => setTooltip(null),
  }

  const tip =
    tooltip &&
    createPortal(
      <div
        className="pointer-events-none fixed z-50 -translate-x-1/2 rounded bg-foreground px-2 py-1 text-xs text-background shadow"
        style={{ left: tooltip.x, top: tooltip.y }}
      >
        {tooltip.label}
      </div>,
      document.body,
    )

  return { handlers, tip }
}
