// Capture the console with headless Chrome over the DevTools protocol.
//
//   node scripts/screenshot.mjs docs/screenshots/000-bootstrap.png             # dev server
//   node scripts/screenshot.mjs out.png http://localhost:3100/                # another port
//   CHROME=/usr/bin/google-chrome WAIT_MS=30000 node scripts/screenshot.mjs out.png
//   EVAL="document.querySelector('button').click()" node scripts/screenshot.mjs out.png  # act first
//   WIDTH=390 HEIGHT=844 node scripts/screenshot.mjs phone.png   # phone-sized viewport
//   DPR=2 node scripts/screenshot.mjs retina.png                  # high-DPI rendering
//   TOUCH=1 node scripts/screenshot.mjs phone.png                 # a touch screen: no hover, coarse pointer
import { spawn } from 'node:child_process'
import { writeFile } from 'node:fs/promises'

const [output, url = 'http://localhost:3000/'] = process.argv.slice(2)
if (!output) {
  console.error('usage: screenshot.mjs <output.png> [url]')
  process.exit(2)
}
const chrome = process.env.CHROME ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const waitMs = Number(process.env.WAIT_MS ?? 3_000)
const evalJs = process.env.EVAL
const width = Number(process.env.WIDTH ?? 1400)
const height = Number(process.env.HEIGHT ?? 900)
const dpr = Number(process.env.DPR ?? 1)
const touch = process.env.TOUCH === '1'

const browser = spawn(chrome, [
  '--headless=new',
  '--remote-debugging-port=0',
  '--use-angle=swiftshader',
  '--enable-unsafe-swiftshader',
  '--ignore-gpu-blocklist',
  `--window-size=${width},${height}`,
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

const port = new URL(wsUrl).port
const targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json()
const page = targets.find((t) => t.type === 'page')
const ws = new WebSocket(page.webSocketDebuggerUrl)
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
  } else if (message.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(message.params.type)) {
    console.error(
      `page console.${message.params.type}:`,
      message.params.args.map((a) => a.description ?? a.value).join(' '),
    )
  }
})
const send = (method, params = {}) =>
  new Promise((resolve, reject) => {
    const id = nextId++
    pending.set(id, (m) => (m.error ? reject(new Error(m.error.message)) : resolve(m.result)))
    ws.send(JSON.stringify({ id, method, params }))
  })

try {
  await send('Runtime.enable')
  await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: dpr, mobile: width < 800 })
  // Touch emulation is what makes the page's `hover: none` and `pointer: coarse` queries true.
  if (touch) await send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 })
  await send('Page.navigate', { url })
  await new Promise((resolve) => setTimeout(resolve, waitMs))
  if (evalJs) {
    await send('Runtime.evaluate', { expression: evalJs, awaitPromise: true })
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
  const { data } = await send('Page.captureScreenshot', { format: 'png' })
  await writeFile(output, Buffer.from(data, 'base64'))
  console.log(`Saved ${output}`)
} finally {
  ws.close()
  browser.kill()
}
