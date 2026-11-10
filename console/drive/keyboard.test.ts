import { twistFromKeys } from './keyboard'

test('keys combine into a unit twist', () => {
  expect(twistFromKeys([])).toEqual({ linear: 0, angular: 0 })
  expect(twistFromKeys(['KeyW'])).toEqual({ linear: 1, angular: 0 })
  expect(twistFromKeys(['KeyW', 'ArrowUp'])).toEqual({ linear: 1, angular: 0 })
  expect(twistFromKeys(['KeyS', 'KeyA'])).toEqual({ linear: -1, angular: 1 })
  expect(twistFromKeys(['KeyA', 'KeyD'])).toEqual({ linear: 0, angular: 0 })
})
