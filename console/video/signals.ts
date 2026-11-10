import type { Envelope } from '@tourian/contract/tourian/v1/transport_pb'

// Video messages bypass the store: signaling and frames go straight to whoever is showing the picture.
type Handler = (envelope: Envelope) => void

const handlers = new Set<Handler>()

export function onVideoMessage(handler: Handler): () => void {
  handlers.add(handler)
  return () => handlers.delete(handler)
}

export function dispatchVideoMessage(envelope: Envelope): void {
  for (const handler of handlers) handler(envelope)
}
