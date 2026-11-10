import { create } from '@bufbuild/protobuf'
import { EnvelopeSchema } from '@tourian/contract/tourian/v1/transport_pb'
import { dispatchVideoMessage, onVideoMessage } from './signals'

test('handlers receive every message until they unsubscribe', () => {
  const seen: string[] = []
  const off = onVideoMessage((envelope) => seen.push(envelope.message.case ?? ''))
  dispatchVideoMessage(create(EnvelopeSchema, { message: { case: 'videoFrame', value: { robotId: 'r1' } } }))
  off()
  dispatchVideoMessage(create(EnvelopeSchema, { message: { case: 'videoFrame', value: { robotId: 'r1' } } }))
  expect(seen).toEqual(['videoFrame'])
})
