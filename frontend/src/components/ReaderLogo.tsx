import { getReaderDisplayName } from '@/lib/utils'

interface ReaderLogoProps {
  name: string
  className?: string
}

export function ReaderLogo({ name, className = 'w-7 h-7' }: ReaderLogoProps) {
  return (
    <img
      src={`/readers/${name}.svg`}
      alt={getReaderDisplayName(name)}
      className={className}
      onError={(e) => {
        ;(e.target as HTMLImageElement).style.display = 'none'
      }}
    />
  )
}
