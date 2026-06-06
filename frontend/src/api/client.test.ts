import { beforeEach, describe, expect, it, vi } from 'vitest'

const axiosMocks = vi.hoisted(() => {
  const get = vi.fn()
  return {
    get,
    create: vi.fn(() => ({
      get,
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
      interceptors: {
        response: {
          use: vi.fn(),
        },
      },
    })),
  }
})

vi.mock('axios', () => ({
  default: {
    create: axiosMocks.create,
    isAxiosError: vi.fn(() => false),
  },
}))

import { api } from './client'

describe('transactions API', () => {
  beforeEach(() => {
    axiosMocks.get.mockReset()
  })

  it('includes active filters in search requests', () => {
    api.transactions.search('instamart', {
      page: 1,
      page_size: 50,
      source_type: 'Credit Card',
      bank: 'HDFC',
      date_from: '2026-04-30T18:30:00.000Z',
      date_to: '2026-05-31T18:29:59.999Z',
      sort_dir: 'asc',
    })

    expect(axiosMocks.get).toHaveBeenCalledOnce()
    const requestURL = axiosMocks.get.mock.calls[0][0] as string
    const url = new URL(requestURL, 'http://localhost')

    expect(url.pathname).toBe('/transactions/search')
    expect(url.searchParams.get('q')).toBe('instamart')
    expect(url.searchParams.get('page_size')).toBe('50')
    expect(url.searchParams.get('source_type')).toBe('Credit Card')
    expect(url.searchParams.get('bank')).toBe('HDFC')
    expect(url.searchParams.get('date_from')).toBe('2026-04-30T18:30:00.000Z')
    expect(url.searchParams.get('date_to')).toBe('2026-05-31T18:29:59.999Z')
    expect(url.searchParams.get('sort_dir')).toBe('asc')
  })
})
