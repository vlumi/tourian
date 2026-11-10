import { parseLines } from './api'

test('lines without blanks', () => {
  expect(parseLines('{"a":1}\n\n{"b":2}\n')).toEqual(['{"a":1}', '{"b":2}'])
  expect(parseLines('')).toEqual([])
})
