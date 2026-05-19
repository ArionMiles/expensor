import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Pagination, pageSizeMenuPosition } from './Pagination'

describe('Pagination', () => {
  it('places page size in the footer range and opens a dropdown from the current limit', async () => {
    const user = userEvent.setup()
    const onPageSize = vi.fn()

    render(
      <Pagination
        page={1}
        pageSize={20}
        total={333}
        onPage={() => undefined}
        onPageSize={onPageSize}
      />,
    )

    expect(screen.getByText(/1-20 of 333/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Rows per page: 20' }))
    await user.click(screen.getByRole('menuitem', { name: '50' }))

    expect(onPageSize).toHaveBeenCalledWith(50)
  })

  it('positions the page-size dropdown upward when the trigger is near the viewport bottom', () => {
    const position = pageSizeMenuPosition(
      {
        top: 712,
        bottom: 740,
        left: 120,
        right: 170,
        width: 50,
      },
      { width: 800, height: 760 },
    )

    expect(position.top).toBeUndefined()
    expect(position.bottom).toBeGreaterThan(0)
    expect(position.maxHeight).toBeGreaterThan(0)
  })
})
