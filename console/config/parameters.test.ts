import { formatParameters, parseParameters } from './api'

test('parameters round-trip through lines', () => {
  const text = 'camera_height = 0.3\narena = lab\n\nnot a parameter\n'
  expect(parseParameters(text)).toEqual({ camera_height: '0.3', arena: 'lab' })
  expect(formatParameters({ a: '1', b: 'two words' })).toBe('a = 1\nb = two words')
})
