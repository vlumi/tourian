import { markAcked, markSent } from './latency'

test('round trip is measured per sequence and forgotten after a while', () => {
  markSent(1n, 1000)
  markSent(2n, 1010)
  expect(markAcked(2n, 1035)).toBe(25)
  expect(markAcked(2n, 1040)).toBeUndefined()
  markSent(3n, 7000)
  expect(markAcked(1n, 7001)).toBeUndefined()
})
