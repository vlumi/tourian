import { pickInput } from './input'

const pad = { connected: true, linear: 0, angular: 0, stopPressed: false, engagePressed: false }
const still = { linear: 0, angular: 0 }

test('a moved stick drives, the keys otherwise, and an idle gamepad still names the source', () => {
  expect(pickInput({ ...pad, angular: -0.5 }, { linear: 1, angular: 0 })).toEqual({
    linear: 0,
    angular: -0.5,
    source: 'gamepad',
  })
  expect(pickInput(pad, { linear: 1, angular: 0 })).toEqual({ linear: 1, angular: 0, source: 'keyboard' })
  expect(pickInput(pad, still)).toEqual({ ...still, source: 'gamepad' })
  expect(pickInput({ ...pad, connected: false }, still)).toEqual({ ...still, source: 'none' })
})
