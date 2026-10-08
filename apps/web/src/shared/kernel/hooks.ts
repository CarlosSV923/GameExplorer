import { useEffect, useState } from 'react'

/**
 * True once `flag` has stayed true for `ms`: loading skeletons wait 300 ms
 * so quick answers do not flicker (docs/design-handoff.md §6).
 */
export function useDelayedFlag(flag: boolean, ms = 300): boolean {
  const [shown, setShown] = useState(false)
  useEffect(() => {
    if (!flag) return
    const timer = setTimeout(() => {
      setShown(true)
    }, ms)
    return () => {
      clearTimeout(timer)
      setShown(false)
    }
  }, [flag, ms])
  return flag && shown
}

/** The value, once it has stopped changing for `ms` (search as you type). */
export function useDebounced<T>(value: T, ms = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(value)
    }, ms)
    return () => {
      clearTimeout(timer)
    }
  }, [value, ms])
  return debounced
}

/** Current time, refreshed every `ms` (relative dates, stale uploads). */
export function useNow(ms = 10_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now())
    }, ms)
    return () => {
      clearInterval(timer)
    }
  }, [ms])
  return now
}
