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
      const { observation, trail, engaged } = useStore.getState()
      const position = observation?.pose?.position
      renderWorld(context, width, height, {
        position: position ? { x: position.x, y: position.y } : undefined,
        heading: heading(observation),
        trail,
        active: engaged,
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
