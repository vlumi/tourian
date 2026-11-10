import { create } from '@bufbuild/protobuf'
import { SessionDescriptionSchema, VideoKind, VideoRequestSchema } from '@tourian/contract/tourian/v1/video_pb'
import { send } from '@/platform/connection'
import { candidateMessage, remoteSide, rtcConfiguration } from './peer'
import { onVideoMessage } from './signals'

export type Viewer = { stop: () => void }

// Offers a receive-only connection to the robot's camera and shows the track that comes back.
export function viewWebRtc(robotId: string, video: HTMLVideoElement, onFrame: () => void): Viewer {
  const pc = new RTCPeerConnection(rtcConfiguration)
  const remote = remoteSide(pc)
  pc.addTransceiver('video', { direction: 'recvonly' })
  pc.ontrack = (event) => {
    video.srcObject = event.streams[0] ?? new MediaStream([event.track])
    video.play().catch(() => {})
  }
  pc.onicecandidate = (event) => {
    if (event.candidate) send(candidateMessage(robotId, '', event.candidate))
  }
  const unsubscribe = onVideoMessage((envelope) => {
    const m = envelope.message
    if (m.case === 'sessionDescription' && m.value.robotId === robotId && m.value.type === 'answer')
      void remote.setDescription(m.value)
    if (m.case === 'iceCandidate' && m.value.robotId === robotId) void remote.addCandidate(m.value)
  })
  let frame = 0
  const count = () => {
    onFrame()
    frame = video.requestVideoFrameCallback(count)
  }
  frame = video.requestVideoFrameCallback(count)

  void pc
    .createOffer()
    .then((offer) => pc.setLocalDescription(offer))
    .then(() => {
      const local = pc.localDescription
      if (local)
        send({
          case: 'sessionDescription',
          value: create(SessionDescriptionSchema, { robotId, type: local.type, sdp: local.sdp }),
        })
    })

  return {
    stop: () => {
      unsubscribe()
      video.cancelVideoFrameCallback(frame)
      pc.close()
      video.srcObject = null
    },
  }
}

// Asks the camera for JPEG frames over the platform and shows each as it arrives.
export function viewMjpeg(robotId: string, img: HTMLImageElement, onFrame: () => void): Viewer {
  const request = (enabled: boolean) =>
    send({ case: 'videoRequest', value: create(VideoRequestSchema, { robotId, kind: VideoKind.MJPEG, enabled }) })
  request(true)
  let url: string | undefined
  const unsubscribe = onVideoMessage((envelope) => {
    const m = envelope.message
    if (m.case !== 'videoFrame' || m.value.robotId !== robotId) return
    const next = URL.createObjectURL(new Blob([m.value.jpeg as BlobPart], { type: 'image/jpeg' }))
    img.onload = () => {
      if (url) URL.revokeObjectURL(url)
      url = next
      onFrame()
    }
    img.src = next
  })
  return {
    stop: () => {
      unsubscribe()
      request(false)
      if (url) URL.revokeObjectURL(url)
      img.removeAttribute('src')
    },
  }
}
