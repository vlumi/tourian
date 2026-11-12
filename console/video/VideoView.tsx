'use client'

import { useEffect, useRef, useState } from 'react'
import { useStore, type VideoPath } from '@/platform/store'
import { LendCamera } from './LendCamera'
import { createMeter, sampleIntervalMs, type Summary } from './measure'
import { viewMjpeg, viewWebRtc } from './viewer'

const paths: { path: VideoPath; label: string }[] = [
  { path: 'webrtc', label: 'WebRTC' },
  { path: 'mjpeg', label: 'MJPEG' },
  { path: 'off', label: 'Off' },
]

declare global {
  interface Window {
    tourianLatency?: { path: VideoPath; samples: number[]; summary: () => Summary | undefined }
  }
}

// The robot's camera, over whichever path is chosen, with the frame rate it is arriving at. When measuring, each frame's clock band is read as it is shown and the ages are summarized.
export function VideoView({ measure = false }: { measure?: boolean }) {
  const robotId = useStore((s) => s.selected)
  const camera = useStore((s) => (s.selected ? s.cameras[s.selected] : undefined))
  const videoPath = useStore((s) => s.videoPath)
  const setVideoPath = useStore((s) => s.setVideoPath)
  const videoRef = useRef<HTMLVideoElement>(null)
  const imgRef = useRef<HTMLImageElement>(null)
  const [fps, setFps] = useState<number>()
  const [latency, setLatency] = useState<Summary>()

  useEffect(() => {
    if (!robotId || !camera || videoPath === 'off') return
    let frames = 0
    const meter = measure ? createMeter() : undefined
    if (meter) window.tourianLatency = { path: videoPath, samples: meter.samples, summary: meter.summary }
    const ticker = setInterval(() => {
      setFps(meter ? meter.takeFrames() : frames)
      frames = 0
      if (meter) setLatency(meter.summary())
    }, 1000)
    const onFrame = () => frames++
    const sampler =
      meter &&
      setInterval(() => {
        if (videoPath === 'webrtc' && videoRef.current) meter.sample(videoRef.current, videoRef.current.videoWidth)
        if (videoPath === 'mjpeg' && imgRef.current) meter.sample(imgRef.current, imgRef.current.naturalWidth)
      }, sampleIntervalMs)
    const viewer =
      videoPath === 'webrtc' && videoRef.current
        ? viewWebRtc(robotId, videoRef.current, onFrame)
        : videoPath === 'mjpeg' && imgRef.current
          ? viewMjpeg(robotId, imgRef.current, onFrame)
          : undefined
    return () => {
      clearInterval(ticker)
      if (sampler) clearInterval(sampler)
      viewer?.stop()
      setFps(undefined)
      setLatency(undefined)
    }
  }, [robotId, camera, videoPath, measure])

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
        <span className="value">
          {latency && `age ${latency.median.toFixed(0)} ms, p95 ${latency.p95.toFixed(0)} ms, n=${latency.count} · `}
          {fps === undefined ? '' : `${fps} fps`}
        </span>
      </div>
      <div className="video-frame">
        <video ref={videoRef} autoPlay playsInline muted hidden={videoPath !== 'webrtc' || !camera} />
        <img ref={imgRef} alt="" hidden={videoPath !== 'mjpeg' || !camera} />
        {!camera && robotId && <LendCamera robotId={robotId} />}
      </div>
    </section>
  )
}
