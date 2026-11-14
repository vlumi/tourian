'use client'

import { useSearchParams } from 'next/navigation'
import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { errorMessage } from '@/platform/error'
import { FirstVisit } from './FirstVisit'
import { serveCamera, type CameraState } from '@/video/camera'
import { composeWithClock } from '@/video/compose'
import { watchWorld } from '@/world/watch'

// The page a phone opens to lend its camera to a robot. Starting needs a tap, since browsers only hand out the camera on a gesture. With ?source=world the camera is the robot's own view of the simulated arena, started at once; with ?clock=1 the frames carry a clock band for measuring latency.
export function CameraPage() {
  const params = useSearchParams()
  const clock = params.get('clock') === '1'
  const world = params.get('source') === 'world'
  const secure = useSyncExternalStore(
    subscribeToNothing,
    () => window.isSecureContext,
    () => true,
  )
  const [robotId, setRobotId] = useState(params.get('robot') ?? 'zebes-1')
  const [stream, setStream] = useState<MediaStream>()
  const [error, setError] = useState<string>()
  const [state, setState] = useState<CameraState>()
  const videoRef = useRef<HTMLVideoElement>(null)
  const rawRef = useRef<HTMLVideoElement>(null)

  const lending = world || !!stream

  useEffect(() => {
    const video = videoRef.current
    const raw = rawRef.current
    if (!video || !raw) return
    const watching = world ? watchWorld(robotId) : undefined
    const source = watching?.stream ?? stream
    if (!source) return
    let composed: ReturnType<typeof composeWithClock> | undefined
    if (clock) {
      raw.srcObject = source
      composed = composeWithClock(raw)
      video.srcObject = composed.stream
    } else {
      video.srcObject = source
    }
    const camera = serveCamera(robotId, composed?.stream ?? source, video, setState, world ? 'simulated' : 'phone')
    return () => {
      camera.stop()
      composed?.stop()
      watching?.stop()
      setState(undefined)
    }
  }, [stream, robotId, clock, world])

  const start = async () => {
    try {
      setStream(await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' }, audio: false }))
      setError(undefined)
    } catch (e) {
      setError(errorMessage(e))
    }
  }
  const stop = () => {
    stream?.getTracks().forEach((track) => track.stop())
    setStream(undefined)
  }

  if (!world && !secure) return <FirstVisit />

  return (
    <main className="camera">
      <h1>{world ? 'tourian simulated camera' : 'tourian camera'}</h1>
      <label>
        robot
        <input value={robotId} onChange={(e) => setRobotId(e.target.value)} disabled={lending && !world} />
      </label>
      {world ? null : stream ? (
        <button type="button" onClick={stop}>
          Stop lending
        </button>
      ) : (
        <button type="button" onClick={start} className="engaged">
          Lend this camera
        </button>
      )}
      {error && <p className="error">{error}</p>}
      <video ref={rawRef} autoPlay playsInline muted hidden />
      <video ref={videoRef} autoPlay playsInline muted hidden={!lending} />
      {state && (
        <dl>
          <dt>platform</dt>
          <dd>{state.connected ? 'connected' : 'connecting'}</dd>
          <dt>WebRTC viewers</dt>
          <dd>
            {state.linked} linked of {state.peers}
          </dd>
          <dt>MJPEG viewers</dt>
          <dd>{state.mjpegViewers}</dd>
          <dt>frames sent</dt>
          <dd>{state.framesSent}</dd>
        </dl>
      )}
    </main>
  )
}

// Whether the page may take the camera is known only in the browser; the server renders as if it may.
const subscribeToNothing = () => () => {}
