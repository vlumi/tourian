// Capture the console with headless Chrome.
//
//   node scripts/screenshot.mjs docs/screenshots/000-bootstrap.png             # dev server
//   node scripts/screenshot.mjs out.png http://localhost:3100/                # another port
//   WAIT_MS=10000 node scripts/screenshot.mjs out.png                         # let the page settle longer
//   EVAL="document.querySelector('button').click()" node scripts/screenshot.mjs out.png  # act first; a throw fails the run
//   WIDTH=390 HEIGHT=844 node scripts/screenshot.mjs phone.png   # phone-sized viewport
//   DPR=2 node scripts/screenshot.mjs retina.png                  # high-DPI rendering
//   TOUCH=1 node scripts/screenshot.mjs phone.png                 # a touch screen: no hover, coarse pointer
import { launch, sleep } from './chrome.mjs'

const [output, url = 'http://localhost:1499/'] = process.argv.slice(2)
if (!output) {
  console.error('usage: screenshot.mjs <output.png> [url]')
  process.exit(2)
}
const waitMs = Number(process.env.WAIT_MS ?? 3_000)
const width = Number(process.env.WIDTH ?? 1400)
const height = Number(process.env.HEIGHT ?? 900)

const chrome = await launch({ width, height })
try {
  const page = await chrome.open(url, {
    width,
    height,
    dpr: Number(process.env.DPR ?? 1),
    touch: process.env.TOUCH === '1',
  })
  await sleep(waitMs)
  if (process.env.EVAL) {
    await page.evaluate(process.env.EVAL)
    await sleep(1500)
  }
  await page.screenshot(output)
  console.log(`Saved ${output}`)
} finally {
  await chrome.close()
}
