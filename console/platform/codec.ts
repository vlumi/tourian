import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { EnvelopeSchema, type Envelope } from '@tourian/contract/tourian/v1/transport_pb'

export type Message = Envelope['message']

export function encode(message: Message): Uint8Array<ArrayBuffer> {
  return new Uint8Array(toBinary(EnvelopeSchema, create(EnvelopeSchema, { message })))
}

export function decode(bytes: Uint8Array): Envelope {
  return fromBinary(EnvelopeSchema, bytes)
}
