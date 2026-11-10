import type { Duration } from '@bufbuild/protobuf/wkt'

// Where a moment of the replay falls in a list ordered by offset: the index of the last entry at or before it, or -1 before the first.
export function indexAt<T extends { offset?: Duration }>(items: T[], seconds: number): number {
  let low = 0
  let high = items.length - 1
  let found = -1
  while (low <= high) {
    const mid = (low + high) >> 1
    if (offsetSeconds(items[mid].offset) <= seconds) {
      found = mid
      low = mid + 1
    } else {
      high = mid - 1
    }
  }
  return found
}

export function offsetSeconds(offset?: Duration): number {
  return offset ? Number(offset.seconds) + offset.nanos / 1e9 : 0
}
