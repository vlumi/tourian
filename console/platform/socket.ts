import type { Envelope } from '@tourian/contract/tourian/v1/transport_pb'
import { decode, encode, type Message } from './codec'

const reconnectDelayMs = 1000

export type Socket = { send: (message: Message) => void; close: () => void }

type Handlers = {
  onOpen?: () => void
  onMessage: (envelope: Envelope) => void
  onClose?: () => void
}

const platformPort = 8080

// Where the platform is: served over https, the console is behind the platform and it is the page's own origin; otherwise it is on the page's host at its own port, unless NEXT_PUBLIC_TOURIAN_URL names it.
export function platformBase(): string {
  const page = typeof window === 'undefined' ? undefined : window.location
  if (page?.protocol === 'https:') return `wss://${page.host}`
  const configured = process.env.NEXT_PUBLIC_TOURIAN_URL
  if (configured) return configured.replace(/^http/, 'ws').replace(/\/+$/, '')
  return `ws://${page?.hostname ?? 'localhost'}:${platformPort}`
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
