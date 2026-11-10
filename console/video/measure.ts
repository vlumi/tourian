import { ageOf, now, readBand, scratchContext } from './clock'

export type Summary = { count: number; median: number; p95: number; min: number; max: number }

const plausibleMs = 10_000

export const sampleIntervalMs = 4

// Collects the age of every new frame that carries a clock band, and summarizes it. Sampling is driven by a timer rather than by the browser painting, so it works in a window nobody looks at; a frame counts once, when its stamp first appears.
export function createMeter() {
  const scratch = scratchContext()
  const samples: number[] = []
  let lastStamp: number | undefined
  let frames = 0
  return {
    samples,
    sample(source: CanvasImageSource, width: number): void {
      if (!scratch) return
      const stamp = readBand(source, width, scratch)
      if (stamp === undefined || stamp === lastStamp) return
      lastStamp = stamp
      frames++
      const age = ageOf(stamp, now())
      if (age < plausibleMs) samples.push(age)
    },
    takeFrames(): number {
      const count = frames
      frames = 0
      return count
    },
    summary(): Summary | undefined {
      return summarize(samples)
    },
  }
}

export function summarize(samples: number[]): Summary | undefined {
  if (samples.length === 0) return undefined
  const sorted = [...samples].sort((a, b) => a - b)
  const at = (q: number) => sorted[Math.min(sorted.length - 1, Math.floor(q * sorted.length))]
  return { count: sorted.length, median: at(0.5), p95: at(0.95), min: sorted[0], max: sorted[sorted.length - 1] }
}
