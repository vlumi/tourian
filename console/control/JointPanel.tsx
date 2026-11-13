'use client'

import { useStore } from '@/platform/store'
import { Row } from '@/telemetry/Row'

const reach = 3.14

// An arm's joints: where each is and a slider for where it should go. The targets are sent while the operator is engaged, and start from where the joints are.
export function JointPanel() {
  const robot = useStore((s) => (s.selected ? s.robots[s.selected] : undefined))
  const joints = useStore((s) => s.observation?.joints)
  const targets = useStore((s) => s.targets)
  const setTarget = useStore((s) => s.setTarget)
  if (!robot) return null

  return (
    <section className="joints">
      <h2>Joints</h2>
      {robot.jointNames.map((name, i) => (
        <Row
          key={name}
          label={name}
          spark={
            <input
              type="range"
              min={-reach}
              max={reach}
              step={0.01}
              value={targets?.[i] ?? 0}
              disabled={!targets}
              aria-label={`${name} target`}
              onChange={(e) => setTarget(i, Number(e.target.value))}
            />
          }
        >
          {joints?.position[i] === undefined ? '–' : `${joints.position[i].toFixed(2)} rad`}
        </Row>
      ))}
      <p className="hint">Engage to drive the joints to their sliders.</p>
    </section>
  )
}
