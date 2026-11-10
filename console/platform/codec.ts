import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import type { Heartbeat, Stop, VelocityCommand } from '@tourian/contract/tourian/v1/robot_pb'
import { EnvelopeSchema, type Envelope } from '@tourian/contract/tourian/v1/transport_pb'

export type Outbound =
  | { case: 'velocityCommand'; value: VelocityCommand }
  | { case: 'heartbeat'; value: Heartbeat }
  | { case: 'stop'; value: Stop }

export function encode(message: Outbound): Uint8Array {
  return toBinary(EnvelopeSchema, create(EnvelopeSchema, { message }))
}

export function decode(bytes: Uint8Array): Envelope {
  return fromBinary(EnvelopeSchema, bytes)
}
