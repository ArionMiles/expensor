import { useEffect, useState } from 'react'
import type { CSSProperties, RefObject } from 'react'

type Options = {
  gap?: number
  maxHeight?: number
  minWidth?: number
}

export function useFixedDropdownPosition<T extends HTMLElement>(
  open: boolean,
  triggerRef: RefObject<T | null>,
  { gap = 4, maxHeight = 192, minWidth = 140 }: Options = {},
) {
  const [style, setStyle] = useState<CSSProperties | null>(null)

  useEffect(() => {
    if (!open) {
      setStyle(null)
      return
    }

    const update = () => {
      const rect = triggerRef.current?.getBoundingClientRect()
      if (!rect) return

      const viewportPadding = 8
      const spaceBelow = window.innerHeight - rect.bottom - viewportPadding
      const spaceAbove = rect.top - viewportPadding
      const openUpward = spaceBelow < 120 && spaceAbove > spaceBelow
      const available = openUpward ? spaceAbove - gap : spaceBelow - gap

      setStyle({
        position: 'fixed',
        ...(openUpward
          ? { bottom: Math.max(viewportPadding, window.innerHeight - rect.top + gap) }
          : { top: rect.bottom + gap }),
        left: Math.max(viewportPadding, rect.left),
        minWidth: Math.max(minWidth, rect.width),
        maxHeight: Math.max(96, Math.min(maxHeight, available)),
      })
    }

    update()
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [gap, maxHeight, minWidth, open, triggerRef])

  return style
}
