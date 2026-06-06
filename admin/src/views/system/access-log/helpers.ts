export function methodClass(method: string): string {
  const map: Record<string, string> = {
    GET: 'text-emerald-600 dark:text-emerald-400',
    POST: 'text-primary',
    PUT: 'text-amber-600 dark:text-amber-400',
    PATCH: 'text-amber-600 dark:text-amber-400',
    DELETE: 'text-destructive',
  }
  return map[method] || ''
}

export function statusVariant(
  code: number,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (code >= 200 && code < 300) return 'default'
  if (code >= 400 && code < 500) return 'secondary'
  if (code >= 500) return 'destructive'
  return 'outline'
}
