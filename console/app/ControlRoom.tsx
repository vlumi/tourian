'use client'

import { useEffect } from 'react'
import { startDriver, toggleEngaged, emergencyStop } from '@/drive/driver'
import { connect } from '@/platform/connection'
import { useStore } from '@/platform/store'
import { TelemetryPanel } from '@/telemetry/TelemetryPanel'
import { WorldCanvas } from '@/world/WorldCanvas'

export function ControlRoom() {
  const engaged = useStore((s) => s.engaged)
  const robot = useStore((s) => s.selected)

  useEffect(() => {
    const disconnect = connect()
    const stopDriver = startDriver()
    return () => {
      stopDriver()
      disconnect()
    }
  }, [])

  return (
    <div className="room">
      <header>
        <h1>tourian</h1>
        <div className="controls">
          <button type="button" onClick={toggleEngaged} disabled={!robot} className={engaged ? 'engaged' : ''}>
            {engaged ? 'Release' : 'Engage'}
          </button>
          <button type="button" onClick={emergencyStop} disabled={!robot} className="stop">
            Stop
          </button>
        </div>
      </header>
      <WorldCanvas />
      <TelemetryPanel />
    </div>
  )
}
