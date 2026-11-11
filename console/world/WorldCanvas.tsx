'use client'

import { useEffect, useRef } from 'react'
import { heading, positionOf, useStore } from '@/platform/store'
import { renderWorld, startCanvasDrawing } from './render'

export function WorldCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const drawing = startCanvasDrawing(canvas, (context, width, height) => {
      const { observation, observations, selected, trail, engaged } = useStore.getState()
      const others = Object.entries(observations).flatMap(([id, other]) => {
        const position = positionOf(other)
        return id !== selected && position ? [{ position, heading: heading(other) }] : []
      })
      renderWorld(context, width, height, {
        position: positionOf(observation),
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
