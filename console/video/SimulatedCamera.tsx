'use client'

import { useEffect } from 'react'
import { useStore } from '@/platform/store'
import { watchWorld } from '@/world/watch'
import { serveCamera } from './camera'

// Lends the simulated camera of the robot the page chose, from this very page: the world is rendered here, where the tab is in front and keeps drawing, and goes out to the platform like a phone's picture would.
export function SimulatedCamera() {
  const robotId = useStore((s) => s.worldCamera)

  useEffect(() => {
    if (!robotId) return
    const watching = watchWorld(robotId)
    const camera = serveCamera(robotId, watching.stream, watching.canvas, () => {}, 'simulated')
    return () => {
      camera.stop()
      watching.stop()
    }
  }, [robotId])

  return null
}
