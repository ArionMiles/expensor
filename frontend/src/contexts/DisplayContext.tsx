import { useTimeFormat, useTimezone } from '@/api/queries'
import { getBrowserTimezone, normalizeTimezone } from '@/lib/timezone'
import { createContext, useContext } from 'react'

export const TIME_FORMATS = [
  { value: 'HH:mm', label: '02 Jan 2006, 14:30', hour12: false, seconds: false },
  { value: 'HH:mm:ss', label: '02 Jan 2006, 14:30:45', hour12: false, seconds: true },
  { value: 'h:mm a', label: '02 Jan 2006, 02:30 PM', hour12: true, seconds: false },
  { value: 'h:mm:ss a', label: '02 Jan 2006, 02:30:45 PM', hour12: true, seconds: true },
] as const

export type TimeFormatValue = (typeof TIME_FORMATS)[number]['value']

interface DisplayContextValue {
  timezone: string
  timeFormat: TimeFormatValue
}

const DisplayContext = createContext<DisplayContextValue>({
  timezone: getBrowserTimezone(),
  timeFormat: 'HH:mm',
})

export function DisplayProvider({ children }: { children: React.ReactNode }) {
  const { data: tz } = useTimezone()
  const { data: tf } = useTimeFormat()

  return (
    <DisplayContext.Provider
      value={{
        timezone: normalizeTimezone(tz) || getBrowserTimezone(),
        timeFormat: (tf as TimeFormatValue | undefined) ?? 'HH:mm',
      }}
    >
      {children}
    </DisplayContext.Provider>
  )
}

export function useDisplay() {
  return useContext(DisplayContext)
}
