import { readGamepad } from './gamepad'

function pad(axes: number[], pressed: number[] = []): Gamepad {
  const buttons = Array.from({ length: 12 }, (_, i) => ({ pressed: pressed.includes(i), touched: false, value: 0 }))
  return { connected: true, id: 'Test pad', axes, buttons } as unknown as Gamepad
}

test('sticks map to a twist with a deadzone', () => {
  expect(readGamepad([null])).toMatchObject({ connected: false, linear: 0, angular: 0 })
  expect(readGamepad([pad([0, -1, 0, 0])])).toMatchObject({ linear: 1, angular: 0 })
  expect(readGamepad([pad([0, 0.1, 1, 0])])).toMatchObject({ linear: 0, angular: -1 })
})

test('buttons fire once per press', () => {
  expect(readGamepad([pad([0, 0, 0, 0], [1])]).stopPressed).toBe(true)
  expect(readGamepad([pad([0, 0, 0, 0], [1])]).stopPressed).toBe(false)
  expect(readGamepad([pad([0, 0, 0, 0], [])]).stopPressed).toBe(false)
  expect(readGamepad([pad([0, 0, 0, 0], [9])]).engagePressed).toBe(true)
})
