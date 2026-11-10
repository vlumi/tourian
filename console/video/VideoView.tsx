'use client'

import { useEffect, useRef, useState } from 'react'
import { useStore, type VideoPath } from '@/platform/store'
import { viewMjpeg, viewWebRtc } from './viewer'

const paths: { path: VideoPath; label: string }[] = [
  { path: 'webrtc', label: 'WebRTC' },
  { path: 'mjpeg', label: 'MJPEG' },
  { path: 'off', label: 'Off' },
]

// The robot's camera, over whichever path is chosen, with the frame rate it is arriving at.
export function VideoView() {
  const robotId = useStore((s) => s.selected)
  const camera = useStore((s) => (s.selected ? s.cameras[s.selected] : undefined))
  const videoPath = useStore((s) => s.videoPath)
  const setVideoPath = useStore((s) => s.setVideoPath)
  const videoRef = useRef<HTMLVideoElement>(null)
  const imgRef = useRef<HTMLImageElement>(null)
  const [fps, setFps] = useState<number>()

  useEffect(() => {
    if (!robotId || !camera || videoPath === 'off') return
    let frames = 0
    const ticker = setInterval(() => {
      setFps(frames)
      frames = 0
    }, 1000)
    const onFrame = () => frames++
    const viewer =
      videoPath === 'webrtc' && videoRef.current
        ? viewWebRtc(robotId, videoRef.current, onFrame)
        : videoPath === 'mjpeg' && imgRef.current
          ? viewMjpeg(robotId, imgRef.current, onFrame)
          : undefined
    return () => {
      clearInterval(ticker)
      viewer?.stop()
      setFps(undefined)
    }
  }, [robotId, camera, videoPath])

  return (
    <section className="video">
      <div className="video-bar">
        <span className="label">camera</span>
        <span>{camera ? camera.name || 'camera' : 'none'}</span>
        <span className="video-paths">
          {paths.map(({ path, label }) => (
            <button
              key={path}
              type="button"
              className={path === videoPath ? 'engaged' : ''}
              onClick={() => setVideoPath(path)}
            >
              {label}
            </button>
          ))}
        </span>
        <span className="value">{fps === undefined ? '' : `${fps} fps`}</span>
      </div>
      <div className="video-frame">
        <video ref={videoRef} autoPlay playsInline muted hidden={videoPath !== 'webrtc' || !camera} />
        <img ref={imgRef} alt="" hidden={videoPath !== 'mjpeg' || !camera} />
        {!camera && <p className="video-empty">No camera. Open /camera on a phone to lend it one.</p>}
      </div>
    </section>
  )
}
