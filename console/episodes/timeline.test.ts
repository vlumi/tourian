import { create } from '@bufbuild/protobuf'
import { DurationSchema } from '@bufbuild/protobuf/wkt'
import { indexAt, offsetSeconds } from './timeline'

const at = (seconds: number) => ({
  offset: create(DurationSchema, { seconds: BigInt(Math.floor(seconds)), nanos: Math.round((seconds % 1) * 1e9) }),
})

test('finds the entry in force at a moment', () => {
  const items = [at(0), at(0.05), at(0.1), at(1.5)]
  expect(indexAt(items, -1)).toBe(-1)
  expect(indexAt(items, 0)).toBe(0)
  expect(indexAt(items, 0.07)).toBe(1)
  expect(indexAt(items, 0.1)).toBe(2)
  expect(indexAt(items, 99)).toBe(3)
  expect(indexAt([], 1)).toBe(-1)
  expect(offsetSeconds(at(1.5).offset)).toBeCloseTo(1.5)
  expect(offsetSeconds(undefined)).toBe(0)
})
