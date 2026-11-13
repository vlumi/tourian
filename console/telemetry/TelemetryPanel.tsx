'use client'

import { platformBase } from '@/platform/socket'
import { batteryLabel, haltLabel, heading, takesJoints, useStore } from '@/platform/store'
import { ControlPanel } from '@/control/ControlPanel'
import { JointPanel } from '@/control/JointPanel'
import { Row } from './Row'
import { Sparkline } from './Sparkline'

export function TelemetryPanel() {
  const status = useStore((s) => s.status)
  const robot = useStore((s) => (s.selected ? s.robots[s.selected] : undefined))
  const observation = useStore((s) => s.observation)
  const history = useStore((s) => s.history)
  const latencyMs = useStore((s) => s.latencyMs)
  const engaged = useStore((s) => s.engaged)
  const stopped = useStore((s) => s.stopped)
  const input = useStore((s) => s.input)

  const halt = haltLabel(observation?.haltReason)
  const limits = robot?.velocityLimits
  const arm = takesJoints(robot)

  return (
    <aside className="telemetry">
      <section>
        <h2>Platform</h2>
        <Row label="link">
          <span className={`status status-${status}`}>{status}</span>
        </Row>
        {status !== 'connected' && <p className="hint">trying {platformBase()}</p>}
        <Row label="robot">{robot ? robot.name : 'none'}</Row>
        <Row label="control">
          {stopped ? (
            <span className="status status-stopped">stopped</span>
          ) : engaged ? (
            <span className="status status-engaged">engaged</span>
          ) : (
            'released'
          )}
        </Row>
        <Row label="halt">{halt ?? '–'}</Row>
        <Row label="input">{input.source}</Row>
      </section>

      <ControlPanel />

      <section>
        <h2>Robot</h2>
        <Row label="x">{format(observation?.pose?.position?.x, 'm')}</Row>
        <Row label="y">{format(observation?.pose?.position?.y, 'm')}</Row>
        <Row label="heading">{observation ? format((heading(observation) * 180) / Math.PI, '°', 0) : '–'}</Row>
        <Row
          label="speed"
          spark={<Sparkline values={history.speed} min={-(limits?.maxLinear ?? 1)} max={limits?.maxLinear ?? 1} />}
        >
          {format(observation?.velocity?.linear?.x, 'm/s')}
        </Row>
        <Row
          label="turn"
          spark={<Sparkline values={history.angular} min={-(limits?.maxAngular ?? 1)} max={limits?.maxAngular ?? 1} />}
        >
          {format(observation?.velocity?.angular?.z, 'rad/s')}
        </Row>
        <Row label="battery" spark={<Sparkline values={history.battery} min={0} max={1} />}>
          {batteryLabel(observation)}
        </Row>
        {!arm &&
          observation?.joints?.name.map((name, i) => (
            <Row key={name} label={name}>
              {format(observation.joints?.velocity[i], 'rad/s')}
            </Row>
          ))}
      </section>

      {arm && <JointPanel />}

      <section>
        <h2>Link</h2>
        <Row label="latency" spark={<Sparkline values={history.latency} min={0} />}>
          {latencyMs === undefined ? '–' : `${latencyMs.toFixed(1)} ms`}
        </Row>
        <Row label="sequence">{observation ? String(observation.lastCommandSequence) : '–'}</Row>
      </section>

      <section className="keys">
        <h2>Keys</h2>
        <p>
          <kbd>Enter</kbd> engage or release · <kbd>Space</kbd> emergency stop · <kbd>W</kbd>
          <kbd>A</kbd>
          <kbd>S</kbd>
          <kbd>D</kbd> drive
        </p>
        <p>Gamepad: left stick drives, right stick turns, Start engages, B stops.</p>
      </section>
    </aside>
  )
}

function format(value: number | undefined, unit: string, digits = 2): string {
  return value === undefined ? '–' : `${value.toFixed(digits)} ${unit}`
}
