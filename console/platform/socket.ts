import type { Envelope } from '@tourian/contract/tourian/v1/transport_pb'
import { decode, encode, type Message } from './codec'

const reconnectDelayMs = 1000

export type Socket = { send: (message: Message) => void; close: () => void }

type Handlers = {
  onOpen?: () => void
  onMessage: (envelope: Envelope) => void
  onClose?: () => void
}

const consolePort = '1499'
const platformPort = '8080'

// Where the platform is: the console is served behind it, so the platform is the page's own origin. Opened on the console's own port instead, the page assumes the platform at its default port on the same host.
export function platformBase(): string {
  const page = typeof window === 'undefined' ? undefined : window.location
  if (!page) return `ws://localhost:${platformPort}`
  const scheme = page.protocol === 'https:' ? 'wss' : 'ws'
  if (page.port === consolePort) return `${scheme}://${page.hostname}:${platformPort}`
  return `${scheme}://${page.host}`
}

// Keeps a WebSocket to the platform open, reconnecting after a pause, until closed.
export function openSocket(url: string, handlers: Handlers): Socket {
  let closed = false
  let ws: WebSocket | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  const open = () => {
    ws = new WebSocket(url)
    ws.binaryType = 'arraybuffer'
    ws.onopen = () => handlers.onOpen?.()
    ws.onmessage = (event: MessageEvent<ArrayBuffer>) => handlers.onMessage(decode(new Uint8Array(event.data)))
    ws.onclose = () => {
      ws = undefined
      handlers.onClose?.()
      if (!closed) timer = setTimeout(open, reconnectDelayMs)
    }
  }
  open()

  return {
    send: (message) => {
      if (ws?.readyState === WebSocket.OPEN) ws.send(encode(message))
    },
    close: () => {
      closed = true
      if (timer) clearTimeout(timer)
      ws?.close()
    },
  }
}
