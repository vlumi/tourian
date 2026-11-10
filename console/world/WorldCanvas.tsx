'use client'

import { useEffect, useRef } from 'react'
import { heading, useStore } from '@/platform/store'
import { renderWorld, startCanvasDrawing } from './render'

export function WorldCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const drawing = startCanvasDrawing(canvas, (context, width, height) => {
      const { observation, observations, selected, trail, engaged } = useStore.getState()
      const position = observation?.pose?.position
      const others = Object.entries(observations).flatMap(([id, other]) => {
        const p = other.pose?.position
        return id !== selected && p ? [{ position: { x: p.x, y: p.y }, heading: heading(other) }] : []
      })
      renderWorld(context, width, height, {
        position: position ? { x: position.x, y: position.y } : undefined,
        heading: heading(observation),
        trail,
        active: engaged,
        others,
      })
    })
    const unsubscribe = useStore.subscribe(drawing.schedule)
    return () => {
      unsubscribe()
      drawing.stop()
    }
  }, [])

  return <canvas ref={canvasRef} className="world" aria-label="The world and the robot" />
}
