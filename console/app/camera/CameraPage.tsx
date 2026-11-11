'use client'

import { useSearchParams } from 'next/navigation'
import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '@/platform/error'
import { serveCamera, type CameraState } from '@/video/camera'
import { composeWithClock } from '@/video/compose'

// The page a phone opens to lend its camera to a robot. Starting needs a tap, since browsers only hand out the camera on a gesture. With ?clock=1 the frames carry a clock band for measuring latency.
export function CameraPage() {
  const params = useSearchParams()
  const clock = params.get('clock') === '1'
  const [robotId, setRobotId] = useState(params.get('robot') ?? 'zebes-1')
  const [stream, setStream] = useState<MediaStream>()
  const [error, setError] = useState<string>()
  const [state, setState] = useState<CameraState>()
  const videoRef = useRef<HTMLVideoElement>(null)
  const rawRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    const raw = rawRef.current
    if (!stream || !video || !raw) return
    let composed: ReturnType<typeof composeWithClock> | undefined
    if (clock) {
      raw.srcObject = stream
      composed = composeWithClock(raw)
      video.srcObject = composed.stream
    } else {
      video.srcObject = stream
    }
    const camera = serveCamera(robotId, composed?.stream ?? stream, video, setState)
    return () => {
      camera.stop()
      composed?.stop()
      setState(undefined)
    }
  }, [stream, robotId, clock])

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

  return (
    <main className="camera">
      <h1>tourian camera</h1>
      <label>
        robot
        <input value={robotId} onChange={(e) => setRobotId(e.target.value)} disabled={!!stream} />
      </label>
      {stream ? (
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
      <video ref={videoRef} autoPlay playsInline muted hidden={!stream} />
      {state && (
        <dl>
          <dt>platform</dt>
          <dd>{state.connected ? 'connected' : 'connecting'}</dd>
          <dt>WebRTC viewers</dt>
          <dd>{state.peers}</dd>
          <dt>MJPEG viewers</dt>
          <dd>{state.mjpegViewers}</dd>
          <dt>frames sent</dt>
          <dd>{state.framesSent}</dd>
        </dl>
      )}
    </main>
  )
}
