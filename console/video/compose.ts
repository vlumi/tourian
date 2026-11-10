import { drawBand, now } from './clock'

// Repaints the camera's frames onto a canvas with the clock band and streams the canvas instead, so what the viewer receives says when it was composed.
export function composeWithClock(source: HTMLVideoElement): { stream: MediaStream; stop: () => void } {
  const canvas = document.createElement('canvas')
  canvas.width = 640
  canvas.height = 480
  const context = canvas.getContext('2d')
  let frame = 0
  const paint = () => {
    if (context && source.videoWidth) {
      if (canvas.width !== source.videoWidth || canvas.height !== source.videoHeight) {
        canvas.width = source.videoWidth
        canvas.height = source.videoHeight
      }
      context.drawImage(source, 0, 0)
      drawBand(context, canvas.width, now())
    }
    frame = source.requestVideoFrameCallback(paint)
  }
  frame = source.requestVideoFrameCallback(paint)
  return {
    stream: canvas.captureStream(),
    stop: () => source.cancelVideoFrameCallback(frame),
  }
}
