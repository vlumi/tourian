'use client'

import { QRCodeSVG } from 'qrcode.react'
import { useStore } from '@/platform/store'

// How to lend a phone's camera to the robot: the address to open, as a code to scan. The page it opens sees the phone through its first visit.
export function LendCamera({ robotId }: { robotId: string }) {
  const lanUrl = useStore((s) => s.lanUrl)
  if (!lanUrl) return <p className="video-empty">No camera. Open /camera on a phone to lend it one.</p>
  const url = `${lanUrl}/camera?robot=${encodeURIComponent(robotId)}`
  return (
    <div className="lend">
      <QRCodeSVG value={url} size={144} bgColor="transparent" fgColor="#e6f5c8" marginSize={1} />
      <p className="video-empty">
        No camera. Scan to lend a phone's, or open <a href={url}>{url}</a>, or the{' '}
        <a href={`/camera?robot=${encodeURIComponent(robotId)}&source=world`} target="_blank" rel="noreferrer">
          simulated camera
        </a>
        .
      </p>
    </div>
  )
}
