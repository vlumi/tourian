import { ageOf, bitsOf, blocks, stampFrom, stampOf } from './clock'

test('a stamp survives the band', () => {
  for (const time of [0, 1, 123456789, 1_762_750_000_000, 2 ** 30 - 1]) {
    const stamp = stampOf(time)
    const bits = bitsOf(stamp)
    expect(bits).toHaveLength(blocks)
    expect(stampFrom(bits)).toBe(stamp)
  }
})

test('a picture without a clock reads as nothing', () => {
  expect(stampFrom(new Array(blocks).fill(false))).toBeUndefined()
  expect(stampFrom(new Array(blocks).fill(true))).toBeUndefined()
  expect(stampFrom([true, false])).toBeUndefined()
})

test('age allows for the stamp wrapping around', () => {
  const t = 1_762_750_000_000
  expect(ageOf(stampOf(t), t + 85)).toBe(85)
  expect(ageOf(stampOf(2 ** 30 - 10), 2 ** 30 + 5)).toBe(15)
})
