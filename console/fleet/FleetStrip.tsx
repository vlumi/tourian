'use client'

import { ControllerKind } from '@tourian/contract/tourian/v1/control_pb'
import { useEffect, useRef } from 'react'
import { haltLabel, useStore } from '@/platform/store'
import { showJpegFrames } from '@/video/viewer'

// Every robot the platform knows, one card each: its state, who holds it, whether it is being recorded, and its latest picture when one is passing by. Clicking a card selects the robot for driving.
export function FleetStrip() {
  const robots = useStore((s) => s.robots)
  const selected = useStore((s) => s.selected)
  const select = useStore((s) => s.select)
  const ids = Object.keys(robots).sort()
  if (ids.length < 2) return null

  return (
    <nav className="fleet" aria-label="Fleet">
      {ids.map((id) => (
        <FleetCard key={id} robotId={id} selected={id === selected} onSelect={() => select(id)} />
      ))}
    </nav>
  )
}

function FleetCard({ robotId, selected, onSelect }: { robotId: string; selected: boolean; onSelect: () => void }) {
  const robot = useStore((s) => s.robots[robotId])
  const observation = useStore((s) => s.observations[robotId])
  const control = useStore((s) => s.controls[robotId])
  const episode = useStore((s) => s.episodes[robotId])
  const consoleId = useStore((s) => s.consoleId)
  const imgRef = useRef<HTMLImageElement>(null)

  useEffect(() => {
    const img = imgRef.current
    if (img) return showJpegFrames(robotId, img)
  }, [robotId])

  const halt = haltLabel(observation?.haltReason)
  const holder = control?.holder
  return (
    <button type="button" className={`fleet-card ${selected ? 'selected' : ''}`} onClick={onSelect}>
      <img ref={imgRef} alt="" />
      <span className="fleet-name">{robot?.name ?? robotId}</span>
      <span className="fleet-line">
        {holder
          ? holder.kind === ControllerKind.OPERATOR
            ? holder.id === consoleId
              ? 'you'
              : 'operator'
            : holder.name
          : 'nobody'}
        {episode?.recording && <span className="status status-stopped">rec</span>}
      </span>
      <span className="fleet-line">
        {halt ?? 'driving'} · {observation?.battery ? `${(observation.battery.percentage * 100).toFixed(0)} %` : '–'}
      </span>
    </button>
  )
}
