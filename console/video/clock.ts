// A clock painted into the picture: a band of black and white blocks across the top encoding the time the frame was composed, so a viewer on the same machine can read how old each frame is when it is shown.

export const blocks = 32
export const bandHeight = 24
const stampBitCount = blocks - 2
const modulus = 2 ** stampBitCount

export function now(): number {
  return performance.timeOrigin + performance.now()
}

export function stampOf(timeMs: number): number {
  return Math.round(timeMs) % modulus
}

// The first block is always white and the last always black, so a picture without a clock is not mistaken for one.
export function bitsOf(stamp: number): boolean[] {
  const bits = [true]
  for (let i = stampBitCount - 1; i >= 0; i--) bits.push(Math.floor(stamp / 2 ** i) % 2 === 1)
  bits.push(false)
  return bits
}

export function stampFrom(bits: boolean[]): number | undefined {
  if (bits.length !== blocks || !bits[0] || bits[blocks - 1]) return undefined
  let stamp = 0
  for (let i = 1; i < blocks - 1; i++) stamp = stamp * 2 + (bits[i] ? 1 : 0)
  return stamp
}

// How much later than the stamp the given time is, in ms, allowing for the stamp wrapping around.
export function ageOf(stamp: number, timeMs: number): number {
  return (((stampOf(timeMs) - stamp) % modulus) + modulus) % modulus
}

export function drawBand(context: CanvasRenderingContext2D, width: number, timeMs: number): void {
  const bits = bitsOf(stampOf(timeMs))
  const blockWidth = width / blocks
  bits.forEach((bit, i) => {
    context.fillStyle = bit ? '#fff' : '#000'
    context.fillRect(Math.floor(i * blockWidth), 0, Math.ceil(blockWidth), bandHeight)
  })
}

// Reads the band back from a picture by scaling its middle row down to one pixel per block.
export function readBand(
  source: CanvasImageSource,
  sourceWidth: number,
  scratch: CanvasRenderingContext2D,
): number | undefined {
  if (sourceWidth === 0) return undefined
  scratch.drawImage(source, 0, bandHeight / 4, sourceWidth, bandHeight / 2, 0, 0, blocks, 1)
  const data = scratch.getImageData(0, 0, blocks, 1).data
  const bits: boolean[] = []
  for (let i = 0; i < blocks; i++) bits.push(data[i * 4] > 128)
  return stampFrom(bits)
}

export function scratchContext(): CanvasRenderingContext2D | undefined {
  const canvas = document.createElement('canvas')
  canvas.width = blocks
  canvas.height = 1
  const context = canvas.getContext('2d', { willReadFrequently: true })
  if (context) context.imageSmoothingEnabled = true
  return context ?? undefined
}
