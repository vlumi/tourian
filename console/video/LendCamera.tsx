'use client'

import { QRCodeSVG } from 'qrcode.react'
import { useStore } from '@/platform/store'

// How to give the robot eyes: a phone's camera, by the address to open as a code to scan, whose page sees the phone through its first visit; or the simulated camera, rendered in this very page and lent from it, since a browser stops drawing in a tab that is not in front.
export function LendCamera({ robotId }: { robotId: string }) {
  const lanUrl = useStore((s) => s.lanUrl)
  const lendWorld = useStore((s) => s.lendWorld)
  if (!lanUrl) return <p className="video-empty">No camera. Open /camera on a phone to lend it one.</p>
  const url = `${lanUrl}/camera?robot=${encodeURIComponent(robotId)}`
  return (
    <div className="lend">
      <QRCodeSVG value={url} size={144} bgColor="transparent" fgColor="#e6f5c8" marginSize={1} />
      <p className="video-empty">
        No camera. Scan to lend a phone's, or open <a href={url}>{url}</a>.
      </p>
      <button type="button" onClick={() => lendWorld(robotId)}>
        Lend the simulated camera
      </button>
    </div>
  )
}
