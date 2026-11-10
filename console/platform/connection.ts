import type { Envelope } from '@tourian/contract/tourian/v1/transport_pb'
import { markAcked } from '@/drive/latency'
import { dispatchVideoMessage } from '@/video/signals'
import type { Message } from './codec'
import { openSocket, platformBase, type Socket } from './socket'
import { useStore } from './store'

let socket: Socket | undefined

// Keeps the console's connection to the platform open until the returned function is called.
export function connect(url = platformBase() + '/console'): () => void {
  useStore.getState().setStatus('connecting')
  socket = openSocket(url, {
    onOpen: () => useStore.getState().setStatus('connected'),
    onMessage: receive,
    onClose: () => useStore.getState().setStatus('disconnected'),
  })
  return () => {
    socket?.close()
    socket = undefined
  }
}

export function send(message: Message): void {
  socket?.send(message)
}

function receive(envelope: Envelope): void {
  const store = useStore.getState()
  switch (envelope.message.case) {
    case 'capabilities':
      store.addRobot(envelope.message.value)
      break
    case 'observation':
      store.observe(envelope.message.value)
      break
    case 'commandAck': {
      const rtt = markAcked(envelope.message.value.sequence)
      if (rtt !== undefined) store.recordLatency(rtt)
      break
    }
    case 'cameraAnnounce':
      store.addCamera(envelope.message.value)
      break
    case 'episodeStatus':
      store.setEpisode(envelope.message.value)
      break
    case 'sessionDescription':
    case 'iceCandidate':
    case 'videoFrame':
      dispatchVideoMessage(envelope)
      break
  }
}
