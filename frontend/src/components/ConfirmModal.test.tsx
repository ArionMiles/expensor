import { fireEvent, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ConfirmModal } from './ConfirmModal'
import { renderWithProviders } from '@/test/render'

function renderModal() {
  const onConfirm = vi.fn()
  const onCancel = vi.fn()

  renderWithProviders(
    <ConfirmModal
      title="Delete label"
      message="This cannot be undone."
      confirmLabel="Delete"
      variant="destructive"
      onConfirm={onConfirm}
      onCancel={onCancel}
    />,
  )

  return { onConfirm, onCancel }
}

describe('ConfirmModal', () => {
  it('renders as a named modal dialog', () => {
    renderModal()

    expect(screen.getByRole('dialog', { name: 'Delete label' })).toHaveAttribute(
      'aria-modal',
      'true',
    )
  })

  it('calls the confirm callback', async () => {
    const user = userEvent.setup()
    const { onConfirm } = renderModal()

    await user.click(screen.getByRole('button', { name: 'Delete' }))

    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls the cancel callback from the cancel button', async () => {
    const user = userEvent.setup()
    const { onCancel } = renderModal()

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('closes on escape', () => {
    const { onCancel } = renderModal()

    fireEvent.keyDown(document, { key: 'Escape' })

    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('closes on overlay mouse down', () => {
    const { onCancel } = renderModal()
    const dialogSurface = screen.getByRole('heading', { name: 'Delete label' }).closest('div')
    const overlay = dialogSurface?.parentElement

    expect(overlay).not.toBeNull()

    fireEvent.mouseDown(overlay!, { target: overlay })

    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
