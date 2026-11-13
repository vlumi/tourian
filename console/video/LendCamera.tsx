'use client'

import { QRCodeSVG } from 'qrcode.react'
import { useStore } from '@/platform/store'

// How to lend a phone's camera to the robot: the address to open, as a code to scan, and the certificate to trust first where the platform serves https.
export function LendCamera({ robotId }: { robotId: string }) {
  const lanUrl = useStore((s) => s.lanUrl)
  if (!lanUrl) return <p className="video-empty">No camera. Open /camera on a phone to lend it one.</p>
  const url = `${lanUrl}/camera?robot=${encodeURIComponent(robotId)}`
  const authority = `${lanUrl}/api/ca.pem`
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
      {lanUrl.startsWith('https:') && (
        <p className="hint">
          First time on a phone: open <a href={authority}>{authority}</a>, install it, and on iOS turn it on under
          Settings › General › About › Certificate Trust Settings.
        </p>
      )}
    </div>
  )
}
