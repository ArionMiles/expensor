export function searchParam(urlString: string, key: string): string | null {
  return new URL(urlString).searchParams.get(key)
}
