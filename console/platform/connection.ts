import { decode, encode, type Outbound } from './codec'
import { markAcked } from '@/drive/latency'
import { useStore } from './store'

const reconnectDelayMs = 1000

let socket: WebSocket | undefined

export function platformUrl(): string {
  return (process.env.NEXT_PUBLIC_TOURIAN_URL ?? 'ws://localhost:8080') + '/console'
}

// Keeps one connection to the platform open for as long as the returned function has not been called.
export function connect(url = platformUrl()): () => void {
  let closed = false
  let timer: ReturnType<typeof setTimeout> | undefined

  const open = () => {
    const store = useStore.getState()
    store.setStatus('connecting')
    const ws = new WebSocket(url)
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => useStore.getState().setStatus('connected')
    ws.onmessage = (event: MessageEvent<ArrayBuffer>) => receive(decode(new Uint8Array(event.data)))
    ws.onclose = () => {
      useStore.getState().setStatus('disconnected')
      socket = undefined
      if (!closed) timer = setTimeout(open, reconnectDelayMs)
    }
    socket = ws
  }
  open()

  return () => {
    closed = true
    if (timer) clearTimeout(timer)
    socket?.close()
    socket = undefined
  }
}

export function send(message: Outbound): void {
  if (socket?.readyState === WebSocket.OPEN) socket.send(new Uint8Array(encode(message)))
}

function receive(envelope: ReturnType<typeof decode>): void {
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
  }
}
