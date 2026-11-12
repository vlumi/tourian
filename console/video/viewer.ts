import { create } from '@bufbuild/protobuf'
import { VideoKind, VideoRequestSchema } from '@tourian/contract/tourian/v1/video_pb'
import { send } from '@/platform/connection'
import { candidateMessage, descriptionMessage, remoteSide, rtcConfiguration } from './peer'
import { onVideoMessage } from './signals'

type Viewer = { stop: () => void }

export type LinkState = RTCPeerConnectionState

const connectWithinMs = 8000

// Offers a receive-only connection to the robot's camera and shows the track that comes back. A link that fails, or is not up within a few seconds, is offered again from scratch; the camera takes a new offer from the same console as a replacement.
export function viewWebRtc(
  robotId: string,
  video: HTMLVideoElement,
  onFrame: () => void,
  onLink?: (state: LinkState) => void,
): Viewer {
  let pc: RTCPeerConnection | undefined
  let remote: ReturnType<typeof remoteSide> | undefined
  let watchdog: ReturnType<typeof setTimeout> | undefined
  let stopped = false

  const unsubscribe = onVideoMessage((envelope) => {
    const m = envelope.message
    if (m.case === 'sessionDescription' && m.value.robotId === robotId && m.value.type === 'answer')
      void remote?.setDescription(m.value)
    if (m.case === 'iceCandidate' && m.value.robotId === robotId) void remote?.addCandidate(m.value)
  })

  const connect = () => {
    pc?.close()
    clearTimeout(watchdog)
    const current = new RTCPeerConnection(rtcConfiguration)
    pc = current
    remote = remoteSide(current)
    current.addTransceiver('video', { direction: 'recvonly' })
    current.ontrack = (event) => {
      video.srcObject = event.streams[0] ?? new MediaStream([event.track])
      video.play().catch(() => {})
    }
    current.onicecandidate = (event) => {
      if (event.candidate) send(candidateMessage(robotId, '', event.candidate))
    }
    current.onconnectionstatechange = () => {
      if (pc !== current) return
      onLink?.(current.connectionState)
      if (current.connectionState === 'connected') clearTimeout(watchdog)
      if (current.connectionState === 'failed') retry()
    }
    onLink?.('connecting')
    watchdog = setTimeout(retry, connectWithinMs)
    void current
      .createOffer()
      .then((offer) => current.setLocalDescription(offer))
      .then(() => {
        if (pc === current && current.localDescription) send(descriptionMessage(robotId, '', current.localDescription))
      })
      .catch(() => {})
  }
  const retry = () => {
    if (!stopped) connect()
  }

  let frame = 0
  const count = () => {
    onFrame()
    frame = video.requestVideoFrameCallback(count)
  }
  frame = video.requestVideoFrameCallback(count)
  connect()

  return {
    stop: () => {
      stopped = true
      clearTimeout(watchdog)
      unsubscribe()
      video.cancelVideoFrameCallback(frame)
      pc?.close()
      video.srcObject = null
    },
  }
}

// Asks the camera for JPEG frames over the platform and shows each as it arrives.
export function viewMjpeg(robotId: string, img: HTMLImageElement, onFrame: () => void): Viewer {
  const request = (enabled: boolean) =>
    send({ case: 'videoRequest', value: create(VideoRequestSchema, { robotId, kind: VideoKind.MJPEG, enabled }) })
  request(true)
  const stop = showJpegFrames(robotId, img, onFrame)
  return {
    stop: () => {
      stop()
      request(false)
      img.removeAttribute('src')
    },
  }
}

// Shows the robot's JPEG frames in img as they pass by, keeping only the one on screen alive, until the returned function is called.
export function showJpegFrames(robotId: string, img: HTMLImageElement, onFrame?: () => void): () => void {
  let url: string | undefined
  const unsubscribe = onVideoMessage((envelope) => {
    const m = envelope.message
    if (m.case !== 'videoFrame' || m.value.robotId !== robotId) return
    const next = URL.createObjectURL(new Blob([m.value.jpeg as BlobPart], { type: 'image/jpeg' }))
    img.onload = () => {
      if (url) URL.revokeObjectURL(url)
      url = next
      onFrame?.()
    }
    img.src = next
  })
  return () => {
    unsubscribe()
    if (url) URL.revokeObjectURL(url)
  }
}
