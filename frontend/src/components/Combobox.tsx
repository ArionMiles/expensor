import { useFixedDropdownPosition } from '@/hooks/useFixedDropdownPosition'
import { cn } from '@/lib/utils'
import type { CSSProperties, KeyboardEvent, ReactNode, RefObject } from 'react'
import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

type ComboboxNavigationOptions = {
  open: boolean
  optionCount: number
  onOpenChange: (open: boolean) => void
  onSelectIndex: (index: number) => void
  onEnterWithoutSelection?: () => void
  onEscape?: () => void
}

type ComboboxPropsInput<T> = Omit<T, 'role' | 'aria-expanded' | 'aria-controls' | 'aria-haspopup'>

export function useComboboxNavigation({
  open,
  optionCount,
  onOpenChange,
  onSelectIndex,
  onEnterWithoutSelection,
  onEscape,
}: ComboboxNavigationOptions) {
  const listboxId = useId()
  const [highlightedIndex, setHighlightedIndex] = useState(-1)

  const resetHighlight = () => setHighlightedIndex(-1)
  const getOptionId = (index: number) => `${listboxId}-option-${index}`
  const activeDescendantId =
    highlightedIndex >= 0 && open ? getOptionId(highlightedIndex) : undefined

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      onOpenChange(false)
      resetHighlight()
      onEscape?.()
      return
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault()
      if (!open) onOpenChange(true)
      setHighlightedIndex((current) => Math.min(current + 1, optionCount - 1))
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      if (!open) onOpenChange(true)
      setHighlightedIndex((current) => Math.max(current - 1, 0))
      return
    }

    if (event.key === 'Enter') {
      if (highlightedIndex >= 0 && highlightedIndex < optionCount) {
        event.preventDefault()
        onSelectIndex(highlightedIndex)
        resetHighlight()
        return
      }
      if (onEnterWithoutSelection) {
        event.preventDefault()
        onEnterWithoutSelection()
      }
    }
  }

  const getComboboxProps = <T extends Record<string, unknown>>(
    {
      listboxVisible = open,
      onKeyDown,
      ...props
    }: ComboboxPropsInput<T> & {
      listboxVisible?: boolean
      onKeyDown?: (event: KeyboardEvent<HTMLElement>) => void
    } = {} as ComboboxPropsInput<T>,
  ) => ({
    ...props,
    role: 'combobox' as const,
    'aria-expanded': open,
    'aria-controls': listboxVisible ? listboxId : undefined,
    'aria-activedescendant': activeDescendantId,
    'aria-haspopup': 'listbox' as const,
    onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
      handleKeyDown(event)
      if (!event.defaultPrevented) onKeyDown?.(event)
    },
  })

  const getOptionProps = (
    index: number,
    {
      selected,
      onMouseDown,
      onMouseEnter,
    }: {
      selected: boolean
      onMouseDown?: () => void
      onMouseEnter?: () => void
    },
  ) => ({
    id: getOptionId(index),
    role: 'option' as const,
    'aria-selected': selected,
    onMouseDown: () => {
      onMouseDown?.()
    },
    onMouseEnter: () => {
      setHighlightedIndex(index)
      onMouseEnter?.()
    },
  })

  return {
    activeDescendantId,
    getComboboxProps,
    getOptionId,
    getOptionProps,
    handleKeyDown,
    highlightedIndex,
    listboxId,
    resetHighlight,
    setHighlightedIndex,
  }
}

type FloatingDropdownProps<T extends HTMLElement> = {
  open: boolean
  anchorRef: RefObject<T | null>
  containerRef?: RefObject<HTMLElement | null>
  onOpenChange: (open: boolean) => void
  children: (style: CSSProperties, setPortalNode: (node: HTMLElement | null) => void) => ReactNode
  gap?: number
  maxHeight?: number
  minWidth?: number
}

export function FloatingDropdown<T extends HTMLElement>({
  open,
  anchorRef,
  containerRef,
  onOpenChange,
  children,
  gap,
  maxHeight,
  minWidth,
}: FloatingDropdownProps<T>) {
  const portalRef = useRef<HTMLElement | null>(null)
  const style = useFixedDropdownPosition(open, anchorRef, { gap, maxHeight, minWidth })

  useEffect(() => {
    if (!open) return

    const handler = (event: MouseEvent) => {
      const target = event.target as Node
      if (containerRef?.current?.contains(target) || portalRef.current?.contains(target)) return
      onOpenChange(false)
    }

    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [containerRef, onOpenChange, open])

  if (!open || !style) return null

  return createPortal(
    children(style, (node) => (portalRef.current = node)),
    document.body,
  )
}

type ComboboxListboxProps<T extends HTMLElement> = Omit<FloatingDropdownProps<T>, 'children'> & {
  listboxId: string
  label: string
  children: ReactNode
  className?: string
}

export function ComboboxListbox<T extends HTMLElement>({
  listboxId,
  label,
  children,
  className,
  ...dropdownProps
}: ComboboxListboxProps<T>) {
  return (
    <FloatingDropdown {...dropdownProps}>
      {(style, setPortalNode) => (
        <ul
          ref={setPortalNode}
          id={listboxId}
          role="listbox"
          aria-label={label}
          style={style}
          className={cn(
            'fixed z-50 overflow-y-auto rounded-md border border-border bg-card shadow-lg',
            className,
          )}
        >
          {children}
        </ul>
      )}
    </FloatingDropdown>
  )
}

export function comboboxOptionClass(highlighted: boolean, selected = false, className?: string) {
  return cn(
    'cursor-pointer px-2 py-1.5 text-xs',
    highlighted && 'bg-accent text-accent-foreground',
    selected && !highlighted && 'text-primary',
    !highlighted && !selected && 'text-foreground hover:bg-accent/50',
    className,
  )
}
