// Measures how old frames are when the console shows them, on both video paths, with a fake camera in headless Chrome.
//
// Opens the camera page with its clock band and the console in measuring mode in one browser, lends the camera, and
// for each path waits for the picture to settle and then collects the age of every frame shown for a while. Prints
// JSON with the summary per path and, when asked, saves a screenshot of the console on the first path.
//
//   node scripts/video-latency.mjs [consoleBase] [robotId] [screenshot.png]
//   SECONDS=10 WARMUP=3 node scripts/video-latency.mjs
import { spawn } from 'node:child_process'
import { writeFile } from 'node:fs/promises'

const [base = 'http://localhost:3100', robotId = 'zebes-1', screenshot] = process.argv.slice(2)
const chrome = process.env.CHROME ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const seconds = Number(process.env.SECONDS ?? 10)
const warmup = Number(process.env.WARMUP ?? 3)
const paths = ['webrtc', 'mjpeg']

const browser = spawn(chrome, [
  '--headless=new',
  '--remote-debugging-port=0',
  '--use-fake-device-for-media-stream',
  '--use-fake-ui-for-media-stream',
  '--autoplay-policy=no-user-gesture-required',
  '--disable-background-timer-throttling',
  '--disable-renderer-backgrounding',
  '--disable-backgrounding-occluded-windows',
  '--window-size=1400,900',
  '--hide-scrollbars',
  'about:blank',
])
const wsUrl = await new Promise((resolve, reject) => {
  browser.stderr.on('data', (chunk) => {
    const match = String(chunk).match(/DevTools listening on (ws:\S+)/)
    if (match) resolve(match[1])
  })
  browser.on('exit', (code) => reject(new Error(`Chrome exited with ${code}`)))
})
const ws = new WebSocket(wsUrl)
await new Promise((resolve) => ws.addEventListener('open', resolve))

let nextId = 1
const pending = new Map()
ws.addEventListener('message', (event) => {
  const message = JSON.parse(event.data)
  if (message.id && pending.has(message.id)) {
    pending.get(message.id)(message)
    pending.delete(message.id)
  } else if (message.method === 'Runtime.exceptionThrown') {
    console.error(
      'page exception:',
      message.params.exceptionDetails.exception?.description ?? message.params.exceptionDetails.text,
    )
  }
})
const send = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const id = nextId++
    pending.set(id, (m) => (m.error ? reject(new Error(`${method}: ${m.error.message}`)) : resolve(m.result)))
    ws.send(JSON.stringify({ id, method, params, sessionId }))
  })
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

// Each page gets a window of its own: a page in a background tab has its timers throttled, which a phone in hand does not.
async function openPage(url) {
  const { targetId } = await send('Target.createTarget', { url: 'about:blank', newWindow: true })
  const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
  await send('Runtime.enable', {}, sessionId)
  await send(
    'Emulation.setDeviceMetricsOverride',
    { width: 1400, height: 900, deviceScaleFactor: 1, mobile: false },
    sessionId,
  )
  await send('Page.navigate', { url }, sessionId)
  return sessionId
}
const evaluate = async (sessionId, expression) =>
  (await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }, sessionId)).result.value

try {
  const camera = await openPage(`${base}/camera?robot=${encodeURIComponent(robotId)}&clock=1`)
  await sleep(2500)
  await evaluate(camera, `document.querySelector('button').click()`)
  await sleep(1500)

  const room = await openPage(`${base}/?measure=1`)
  await sleep(2500)

  const results = {}
  for (const path of paths) {
    await evaluate(
      room,
      `[...document.querySelectorAll('.video-paths button')].find((b) => b.textContent.toLowerCase() === '${path}').click()`,
    )
    await sleep(warmup * 1000)
    await evaluate(room, `window.tourianLatency.samples.length = 0`)
    const before = Date.now()
    await sleep(seconds * 1000)
    const samples = await evaluate(room, `JSON.stringify(window.tourianLatency.samples)`)
    const ages = JSON.parse(samples).sort((a, b) => a - b)
    const at = (q) => ages[Math.min(ages.length - 1, Math.floor(q * ages.length))]
    results[path] = {
      frames: ages.length,
      fps: Number((ages.length / ((Date.now() - before) / 1000)).toFixed(1)),
      medianMs: at(0.5),
      p95Ms: at(0.95),
      minMs: ages[0],
      maxMs: ages[ages.length - 1],
    }
    if (screenshot && path === paths[0]) {
      const { data } = await send('Page.captureScreenshot', { format: 'png' }, room)
      await writeFile(screenshot, Buffer.from(data, 'base64'))
    }
  }
  const version = (await send('Browser.getVersion')).product
  console.log(JSON.stringify({ browser: version, seconds, results }, null, 2))
} finally {
  ws.close()
  browser.kill()
}
