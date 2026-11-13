import { create } from '@bufbuild/protobuf'
import { timestampNow } from '@bufbuild/protobuf/wkt'
import type { Envelope } from '@tourian/contract/tourian/v1/transport_pb'
import { CameraAnnounceSchema, VideoFrameSchema, VideoKind } from '@tourian/contract/tourian/v1/video_pb'
import { openSocket, platformBase, type Socket } from '@/platform/socket'
import { candidateMessage, descriptionMessage, remoteSide, rtcConfiguration } from './peer'

const mjpegIntervalMs = 66
const mjpegWidth = 640
const mjpegQuality = 0.7

export type CameraState = {
  connected: boolean
  peers: number
  linked: number
  mjpegViewers: number
  framesSent: number
}

type Camera = { stop: () => void }

// Lends this device's camera to a robot: answers every console that offers a WebRTC connection, and sends JPEG frames over the platform while any console asks for them.
export function serveCamera(
  robotId: string,
  stream: MediaStream,
  source: HTMLVideoElement,
  onState: (state: CameraState) => void,
  name = 'phone',
  url = platformBase() + '/api/camera',
): Camera {
  const state: CameraState = { connected: false, peers: 0, linked: 0, mjpegViewers: 0, framesSent: 0 }
  const peers = new Map<string, { pc: RTCPeerConnection; remote: ReturnType<typeof remoteSide> }>()
  const mjpegPeers = new Set<string>()
  let sequence = 0n
  let capture: ReturnType<typeof setInterval> | undefined
  const canvas = document.createElement('canvas')

  const update = () =>
    onState({
      ...state,
      peers: peers.size,
      linked: [...peers.values()].filter(({ pc }) => pc.connectionState === 'connected').length,
      mjpegViewers: mjpegPeers.size,
    })

  const socket: Socket = openSocket(url, {
    onOpen: () => {
      state.connected = true
      socket.send({
        case: 'cameraAnnounce',
        value: create(CameraAnnounceSchema, { robotId, name, kinds: [VideoKind.WEBRTC, VideoKind.MJPEG] }),
      })
      update()
    },
    onMessage: (envelope) => void handle(envelope),
    onClose: () => {
      state.connected = false
      closePeers()
      update()
    },
  })

  async function handle(envelope: Envelope) {
    const m = envelope.message
    if (m.case === 'sessionDescription' && m.value.robotId === robotId && m.value.type === 'offer') {
      const peerId = m.value.peerId
      peers.get(peerId)?.pc.close()
      const pc = new RTCPeerConnection(rtcConfiguration)
      const remote = remoteSide(pc)
      peers.set(peerId, { pc, remote })
      for (const track of stream.getTracks()) pc.addTrack(track, stream)
      pc.onicecandidate = (event) => {
        if (event.candidate) socket.send(candidateMessage(robotId, peerId, event.candidate))
      }
      pc.onconnectionstatechange = () => {
        if (['failed', 'closed', 'disconnected'].includes(pc.connectionState)) {
          pc.close()
          if (peers.get(peerId)?.pc === pc) peers.delete(peerId)
        }
        update()
      }
      await remote.setDescription(m.value)
      await pc.setLocalDescription(await pc.createAnswer())
      if (pc.localDescription) socket.send(descriptionMessage(robotId, peerId, pc.localDescription))
      update()
    } else if (m.case === 'iceCandidate' && m.value.robotId === robotId) {
      await peers.get(m.value.peerId)?.remote.addCandidate(m.value)
    } else if (m.case === 'videoRequest' && m.value.robotId === robotId && m.value.kind === VideoKind.MJPEG) {
      if (m.value.enabled) mjpegPeers.add(m.value.peerId)
      else mjpegPeers.delete(m.value.peerId)
      if (mjpegPeers.size > 0 && !capture) capture = setInterval(sendFrame, mjpegIntervalMs)
      if (mjpegPeers.size === 0 && capture) {
        clearInterval(capture)
        capture = undefined
      }
      update()
    }
  }

  let encoding = false
  function sendFrame() {
    if (encoding || source.videoWidth === 0) return
    const scale = Math.min(1, mjpegWidth / source.videoWidth)
    canvas.width = Math.round(source.videoWidth * scale)
    canvas.height = Math.round(source.videoHeight * scale)
    canvas.getContext('2d')?.drawImage(source, 0, 0, canvas.width, canvas.height)
    encoding = true
    canvas.toBlob(
      (blob) => {
        encoding = false
        if (!blob) return
        void blob.arrayBuffer().then((buffer) => {
          sequence += 1n
          socket.send({
            case: 'videoFrame',
            value: create(VideoFrameSchema, {
              header: { stamp: timestampNow(), frameId: 'camera' },
              robotId,
              sequence,
              jpeg: new Uint8Array(buffer),
              width: canvas.width,
              height: canvas.height,
            }),
          })
          state.framesSent++
          update()
        })
      },
      'image/jpeg',
      mjpegQuality,
    )
  }

  function closePeers() {
    for (const { pc } of peers.values()) pc.close()
    peers.clear()
    mjpegPeers.clear()
    if (capture) clearInterval(capture)
    capture = undefined
  }

  update()
  return {
    stop: () => {
      closePeers()
      socket.close()
    },
  }
}
