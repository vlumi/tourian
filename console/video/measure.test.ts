import { summarize } from './measure'

test('summary of the samples', () => {
  expect(summarize([])).toBeUndefined()
  expect(summarize([5])).toEqual({ count: 1, median: 5, p95: 5, min: 5, max: 5 })
  const samples = Array.from({ length: 100 }, (_, i) => 100 - i)
  expect(summarize(samples)).toEqual({ count: 100, median: 51, p95: 96, min: 1, max: 100 })
})
