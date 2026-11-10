'use client'

import { useSearchParams } from 'next/navigation'
import { useEffect, useRef, useState } from 'react'
import { serveCamera, type CameraState } from '@/video/camera'

// The page a phone opens to lend its camera to a robot. Starting needs a tap, since browsers only hand out the camera on a gesture.
export function CameraPage() {
  const params = useSearchParams()
  const [robotId, setRobotId] = useState(params.get('robot') ?? 'zebes-1')
  const [stream, setStream] = useState<MediaStream>()
  const [error, setError] = useState<string>()
  const [state, setState] = useState<CameraState>()
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!stream || !video) return
    video.srcObject = stream
    const camera = serveCamera(robotId, stream, video, setState)
    return () => {
      camera.stop()
      setState(undefined)
    }
  }, [stream, robotId])

  const start = async () => {
    try {
      setStream(await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' }, audio: false }))
      setError(undefined)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
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
