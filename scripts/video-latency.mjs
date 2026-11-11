// Measures how old frames are when the console shows them, on both video paths, with a fake camera in headless Chrome.
//
// Opens the camera page with its clock band and the console in measuring mode, lends the camera, waits for each
// side to be ready, and for each path collects the age of every frame shown for a while. Prints JSON with the
// console's own summary per path and, when asked, saves a screenshot of the console on the first path. A path
// that shows no frames fails the run.
//
//   node scripts/video-latency.mjs [consoleBase] [robotId] [screenshot.png]
//   SECONDS=10 node scripts/video-latency.mjs
import { launch, sleep } from './chrome.mjs'

const [base = 'http://localhost:3100', robotId = 'zebes-1', screenshot] = process.argv.slice(2)
const seconds = Number(process.env.SECONDS ?? 10)
const paths = ['webrtc', 'mjpeg']

const chrome = await launch({ fakeCamera: true })
try {
  const camera = await chrome.open(`${base}/camera?robot=${encodeURIComponent(robotId)}&clock=1`)
  await camera.waitFor(`!!document.querySelector('button')`, { what: 'the camera page' })
  await camera.evaluate(`document.querySelector('button').click()`)
  await camera.waitFor(`document.querySelector('dd')?.textContent === 'connected'`, { what: 'the camera to connect' })

  const room = await chrome.open(`${base}/?measure=1`)
  await room.waitFor(`document.querySelector('.video-paths button') !== null`, { what: 'the console' })

  const results = {}
  for (const path of paths) {
    await room.evaluate(
      `[...document.querySelectorAll('.video-paths button')].find((b) => b.textContent.toLowerCase() === '${path}').click()`,
    )
    await room.waitFor(`window.tourianLatency?.path === '${path}' && window.tourianLatency.samples.length > 0`, {
      what: `frames with a clock over ${path}`,
    })
    await sleep(1000)
    await room.evaluate(`window.tourianLatency.samples.length = 0`)
    const before = Date.now()
    await sleep(seconds * 1000)
    const summary = await room.evaluate(`JSON.stringify(window.tourianLatency.summary())`)
    const stats = JSON.parse(summary)
    if (!stats) throw new Error(`no frames with a clock were shown over ${path}`)
    results[path] = {
      frames: stats.count,
      fps: Number((stats.count / ((Date.now() - before) / 1000)).toFixed(1)),
      medianMs: Math.round(stats.median),
      p95Ms: Math.round(stats.p95),
      minMs: Math.round(stats.min),
      maxMs: Math.round(stats.max),
    }
    if (screenshot && path === paths[0]) await room.screenshot(screenshot)
  }
  console.log(JSON.stringify({ browser: await chrome.version(), seconds, results }, null, 2))
} finally {
  await chrome.close()
}
