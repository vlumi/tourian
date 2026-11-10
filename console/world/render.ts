import type { Point } from '@/platform/store'

const viewMeters = 12
const robotRadius = 0.25

export type WorldState = {
  position?: Point
  heading: number
  trail: Point[]
  active: boolean
  others?: { position: Point; heading: number }[]
}

// The arena from above: a meter grid centered on the origin, the trail driven and the robot with its heading.
export function renderWorld(context: CanvasRenderingContext2D, width: number, height: number, state: WorldState): void {
  const scale = Math.min(width, height) / viewMeters
  const toScreen = (p: Point) => ({ x: width / 2 + p.x * scale, y: height / 2 - p.y * scale })
  const styles = getComputedStyle(context.canvas)
  const color = (name: string) => styles.getPropertyValue(name).trim()

  context.clearRect(0, 0, width, height)

  context.strokeStyle = color('--grid')
  context.lineWidth = 1
  const half = Math.ceil(Math.max(width, height) / scale / 2)
  for (let m = -half; m <= half; m++) {
    const { x } = toScreen({ x: m, y: 0 })
    const { y } = toScreen({ x: 0, y: m })
    context.beginPath()
    context.moveTo(x, 0)
    context.lineTo(x, height)
    context.moveTo(0, y)
    context.lineTo(width, y)
    context.stroke()
  }
  context.strokeStyle = color('--border')
  context.lineWidth = 2
  const origin = toScreen({ x: 0, y: 0 })
  context.beginPath()
  context.moveTo(origin.x, 0)
  context.lineTo(origin.x, height)
  context.moveTo(0, origin.y)
  context.lineTo(width, origin.y)
  context.stroke()

  if (state.trail.length > 1) {
    context.strokeStyle = color('--trail')
    context.lineWidth = 2
    context.beginPath()
    state.trail.forEach((p, i) => {
      const s = toScreen(p)
      if (i === 0) context.moveTo(s.x, s.y)
      else context.lineTo(s.x, s.y)
    })
    context.stroke()
  }

  const r = robotRadius * scale
  const robot = (position: Point, heading: number, fill: string) => {
    const center = toScreen(position)
    context.fillStyle = fill
    context.beginPath()
    context.arc(center.x, center.y, r, 0, 2 * Math.PI)
    context.fill()
    context.strokeStyle = color('--bg')
    context.lineWidth = 3
    context.beginPath()
    context.moveTo(center.x, center.y)
    context.lineTo(center.x + Math.cos(heading) * r * 1.6, center.y - Math.sin(heading) * r * 1.6)
    context.stroke()
  }
  for (const other of state.others ?? []) robot(other.position, other.heading, color('--trail'))
  if (state.position) robot(state.position, state.heading, state.active ? color('--accent') : color('--dim'))
}

// Keeps a canvas sized to its element and the device, and redraws through draw whenever asked.
export function startCanvasDrawing(
  canvas: HTMLCanvasElement,
  draw: (context: CanvasRenderingContext2D, width: number, height: number) => void,
) {
  let frame = 0
  const paint = () => {
    frame = 0
    const context = canvas.getContext('2d')
    if (!context) return
    const dpr = window.devicePixelRatio || 1
    const width = canvas.clientWidth
    const height = canvas.clientHeight
    if (canvas.width !== width * dpr || canvas.height !== height * dpr) {
      canvas.width = width * dpr
      canvas.height = height * dpr
    }
    context.setTransform(dpr, 0, 0, dpr, 0, 0)
    draw(context, width, height)
  }
  const schedule = () => {
    if (!frame) frame = requestAnimationFrame(paint)
  }
  const observer = new ResizeObserver(schedule)
  observer.observe(canvas)
  schedule()
  return {
    schedule,
    stop: () => {
      observer.disconnect()
      if (frame) cancelAnimationFrame(frame)
    },
  }
}
