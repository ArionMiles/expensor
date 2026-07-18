import { beforeEach, describe, expect, it, vi } from 'vitest'

const axiosMocks = vi.hoisted(() => {
  const get = vi.fn()
  const responseUse = vi.fn()
  const isAxiosError = vi.fn(() => false)
  return {
    get,
    responseUse,
    isAxiosError,
    create: vi.fn(() => ({
      get,
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
      interceptors: {
        response: {
          use: responseUse,
        },
      },
    })),
  }
})

vi.mock('axios', () => ({
  default: {
    create: axiosMocks.create,
    isAxiosError: axiosMocks.isAxiosError,
  },
}))

import { api, ApiError } from './client'

const responseErrorHandler = axiosMocks.responseUse.mock.calls[0]?.[1] as (error: unknown) => never

describe('transactions API', () => {
  beforeEach(() => {
    axiosMocks.get.mockReset()
    axiosMocks.isAxiosError.mockReset()
    axiosMocks.isAxiosError.mockReturnValue(false)
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

    expect(url.pathname).toBe('/transactions')
    expect(url.searchParams.get('q')).toBe('instamart')
    expect(url.searchParams.get('page_size')).toBe('50')
    expect(url.searchParams.get('source_type')).toBe('Credit Card')
    expect(url.searchParams.get('bank')).toBe('HDFC')
    expect(url.searchParams.get('date_from')).toBe('2026-04-30T18:30:00.000Z')
    expect(url.searchParams.get('date_to')).toBe('2026-05-31T18:29:59.999Z')
    expect(url.searchParams.get('sort_dir')).toBe('asc')
  })

  it('preserves validation errors and request ID from API errors', () => {
    axiosMocks.isAxiosError.mockReturnValue(true)

    try {
      responseErrorHandler({
        response: {
          status: 422,
          data: {
            message: 'Request validation failed.',
            request_id: '7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3',
            validation_errors: [
              { field: 'email', location: 'body', message: 'must be a valid email address' },
            ],
          },
        },
      })
      expect.unreachable('expected error handler to throw')
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect(error).toMatchObject({
        message: 'Request validation failed.',
        requestID: '7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3',
        validationErrors: [
          { field: 'email', location: 'body', message: 'must be a valid email address' },
        ],
      })
    }
  })
})
