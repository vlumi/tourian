// Headless Chrome over the DevTools protocol, for the scripts that capture or measure the console.
//
//   const chrome = await launch({ fakeCamera: true })
//   const page = await chrome.open(url, { width: 1400, height: 900 })
//   await page.evaluate(`document.title`)
//   await page.waitFor(`document.querySelector('.ready')`, { timeout: 10_000 })
//   await page.screenshot('out.png')
//   await chrome.close()
import { spawn } from 'node:child_process'
import { writeFile } from 'node:fs/promises'

const defaultChrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'

export async function launch({ fakeCamera = false, width = 1400, height = 900 } = {}) {
  const browser = spawn(process.env.CHROME ?? defaultChrome, [
    '--headless=new',
    '--remote-debugging-port=0',
    '--use-angle=swiftshader',
    '--enable-unsafe-swiftshader',
    '--ignore-gpu-blocklist',
    // Every page gets a window of its own and runs at full speed, as a page in hand would; a background tab's timers tick once a second.
    '--disable-background-timer-throttling',
    '--disable-renderer-backgrounding',
    '--disable-backgrounding-occluded-windows',
    '--autoplay-policy=no-user-gesture-required',
    ...(fakeCamera ? ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] : []),
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
  const ws = new WebSocket(wsUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve)
    ws.addEventListener('error', reject)
  })

  let nextId = 1
  const pending = new Map()
  const failAll = (why) => {
    for (const { reject } of pending.values()) reject(new Error(why))
    pending.clear()
  }
  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data)
    if (message.id && pending.has(message.id)) {
      const { resolve, reject } = pending.get(message.id)
      pending.delete(message.id)
      if (message.error) reject(new Error(`${message.error.message}`))
      else resolve(message.result)
    } else if (message.method === 'Runtime.exceptionThrown') {
      const details = message.params.exceptionDetails
      console.error('page exception:', details.exception?.description ?? details.text)
    } else if (message.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(message.params.type)) {
      console.error(
        `page console.${message.params.type}:`,
        message.params.args.map((a) => a.description ?? a.value).join(' '),
      )
    }
  })
  ws.addEventListener('close', () => failAll('DevTools connection closed'))
  ws.addEventListener('error', () => failAll('DevTools connection failed'))
  browser.on('exit', (code) => failAll(`Chrome exited with ${code}`))

  const send = (method, params = {}, sessionId) =>
    new Promise((resolve, reject) => {
      const id = nextId++
      pending.set(id, { resolve, reject })
      ws.send(JSON.stringify({ id, method, params, sessionId }))
    })

  const open = async (url, { width: w = width, height: h = height, dpr = 1, touch = false } = {}) => {
    const { targetId } = await send('Target.createTarget', { url: 'about:blank', newWindow: true })
    const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
    await send('Runtime.enable', {}, sessionId)
    await send('Page.enable', {}, sessionId)
    await send(
      'Emulation.setDeviceMetricsOverride',
      { width: w, height: h, deviceScaleFactor: dpr, mobile: w < 800 },
      sessionId,
    )
    // Touch emulation is what makes the page's `hover: none` and `pointer: coarse` queries true.
    if (touch) await send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 }, sessionId)
    await send('Page.navigate', { url }, sessionId)

    // Evaluates in the page and fails on what the page threw, instead of handing back undefined.
    const evaluate = async (expression) => {
      const { result, exceptionDetails } = await send(
        'Runtime.evaluate',
        { expression, awaitPromise: true, returnByValue: true },
        sessionId,
      )
      if (exceptionDetails) {
        throw new Error(`in the page: ${exceptionDetails.exception?.description ?? exceptionDetails.text}`)
      }
      return result.value
    }
    const waitFor = async (expression, { timeout = 10_000, every = 100, what = expression } = {}) => {
      const deadline = Date.now() + timeout
      while (true) {
        const value = await evaluate(expression)
        if (value) return value
        if (Date.now() > deadline) throw new Error(`timed out after ${timeout} ms waiting for ${what}`)
        await sleep(every)
      }
    }
    const screenshot = async (path) => {
      const { data } = await send('Page.captureScreenshot', { format: 'png' }, sessionId)
      await writeFile(path, Buffer.from(data, 'base64'))
    }
    const mouseMove = (x, y) => send('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y }, sessionId)
    return { evaluate, waitFor, screenshot, mouseMove, sessionId }
  }

  const close = async () => {
    const version = await send('Browser.getVersion').catch(() => ({ product: 'unknown' }))
    ws.close()
    browser.kill()
    return version.product
  }
  const version = async () => (await send('Browser.getVersion')).product

  return { open, send, close, version }
}

export const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
