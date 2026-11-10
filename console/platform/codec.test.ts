import { create } from '@bufbuild/protobuf'
import { StopSchema } from '@tourian/contract/tourian/v1/robot_pb'
import { decode, encode } from './codec'

test('a message survives the wire', () => {
  const bytes = encode({ case: 'stop', value: create(StopSchema, { robotId: 'r1' }) })
  const envelope = decode(bytes)
  expect(envelope.message.case).toBe('stop')
  expect(envelope.message.case === 'stop' && envelope.message.value.robotId).toBe('r1')
})
