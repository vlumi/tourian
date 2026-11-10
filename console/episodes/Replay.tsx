'use client'

import Link from 'next/link'
import { useCallback, useEffect, useRef, useState } from 'react'
import { heading } from '@/platform/store'
import { renderWorld, startCanvasDrawing } from '@/world/render'
import { loadEpisode, pictureUrl, type Recording } from './api'
import { length, started } from './EpisodeList'
import { indexAt, offsetSeconds } from './timeline'

// One episode played back: the world as recorded on the left, the picture in force on the right, scrubbed or played at the speed it happened.
export function Replay({ id }: { id: string }) {
  const [recording, setRecording] = useState<Recording>()
  const [error, setError] = useState<string>()
  const [time, setTime] = useState(0)
  const [playing, setPlaying] = useState(false)
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    loadEpisode(id)
      .then(setRecording)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [id])

  const duration = recording ? offsetSeconds(recording.frames[recording.frames.length - 1]?.offset) : 0
  const frameIndex = recording ? indexAt(recording.frames, time) : -1
  const frame = frameIndex >= 0 ? recording?.frames[frameIndex] : undefined
  const pictureIndex = recording ? indexAt(recording.video, time) : -1
  const picture = pictureIndex >= 0 ? recording?.video[pictureIndex] : undefined

  useEffect(() => {
    if (!playing) return
    let last = performance.now()
    let raf = 0
    const step = (now: number) => {
      setTime((t) => {
        const next = t + (now - last) / 1000
        if (next >= duration) {
          setPlaying(false)
          return duration
        }
        return next
      })
      last = now
      raf = requestAnimationFrame(step)
    }
    raf = requestAnimationFrame(step)
    return () => cancelAnimationFrame(raf)
  }, [playing, duration])

  const draw = useCallback(
    (context: CanvasRenderingContext2D, width: number, height: number) => {
      if (!recording) return
      const trail = recording.frames.slice(0, frameIndex + 1).flatMap((f) => {
        const p = f.observation?.pose?.position
        return p ? [{ x: p.x, y: p.y }] : []
      })
      const position = frame?.observation?.pose?.position
      renderWorld(context, width, height, {
        position: position ? { x: position.x, y: position.y } : undefined,
        heading: heading(frame?.observation),
        trail,
        active: true,
      })
    },
    [recording, frameIndex, frame],
  )
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const drawing = startCanvasDrawing(canvas, draw)
    return drawing.stop
  }, [draw])

  if (error) return <p className="error">{error}</p>
  if (!recording) return <p className="video-empty">Loading…</p>
  const { episode } = recording

  return (
    <main className="replay">
      <header>
        <h1>
          <Link href="/">tourian</Link> · <Link href="/episodes">episodes</Link> · {started(episode)}
        </h1>
        <span className="episode">
          {episode.robotId}
          {episode.robotConfigVersion > 0 && ` v${episode.robotConfigVersion}`} · {episode.task || 'no task'}
          {episode.taskConfigVersion > 0 && ` v${episode.taskConfigVersion}`} · {length(episode)} ·{' '}
          {String(episode.frames)} frames · {String(episode.videoFrames)} pictures
          {episode.bad && ` · bad${episode.note ? `: ${episode.note}` : ''}`}
        </span>
      </header>
      <div className="replay-scene">
        <canvas ref={canvasRef} className="world" aria-label="The world as recorded" />
        <div className="video-frame">
          {picture ? <img src={pictureUrl(id, picture)} alt="" /> : <p className="video-empty">No picture yet</p>}
        </div>
      </div>
      <div className="replay-bar">
        <button type="button" onClick={() => setPlaying((p) => !p)}>
          {playing ? 'Pause' : time >= duration ? 'Replay' : 'Play'}
        </button>
        <input
          type="range"
          min={0}
          max={duration}
          step={0.01}
          value={time}
          onChange={(e) => {
            setPlaying(false)
            setTime(Number(e.target.value))
          }}
          aria-label="Time"
        />
        <span className="value">
          {time.toFixed(2)} / {duration.toFixed(2)} s
        </span>
        <span className="value">
          {frame
            ? `x ${frame.observation?.pose?.position?.x.toFixed(2)} y ${frame.observation?.pose?.position?.y.toFixed(2)} · v ${(frame.observation?.velocity?.linear?.x ?? 0).toFixed(2)} · cmd ${(frame.action?.twist?.linear?.x ?? 0).toFixed(2)} · held by ${frame.controller?.name ?? 'nobody'}`
            : ''}
        </span>
      </div>
    </main>
  )
}
