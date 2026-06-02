import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Pagination } from './Pagination'

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
    expect(screen.getByRole('menu', { name: 'Rows per page' }).parentElement).toBe(document.body)
    await user.click(screen.getByRole('menuitem', { name: '50' }))

    expect(onPageSize).toHaveBeenCalledWith(50)
  })
})
