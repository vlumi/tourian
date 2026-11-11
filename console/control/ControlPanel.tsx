'use client'

import { timestampDate } from '@bufbuild/protobuf/wkt'
import { ControllerKind } from '@tourian/contract/tourian/v1/control_pb'
import { handTo } from '@/drive/driver'
import { useStore } from '@/platform/store'
import { Row } from '@/telemetry/Row'
import { holderLabel, ownOperator } from './holder'

// Who holds the robot and since when, and the controllers it can be handed to. The operator takes over by engaging and hands back by releasing; while another console's operator holds the robot, this one waits.
export function ControlPanel() {
  const control = useStore((s) => s.control)
  const engaged = useStore((s) => s.engaged)
  const consoleId = useStore((s) => s.consoleId)
  const holder = control?.holder
  const mine = ownOperator(holder, consoleId)
  const another = holder?.kind === ControllerKind.OPERATOR && !mine

  return (
    <section className="control">
      <h2>Control</h2>
      <Row label="holder">
        {holder ? (
          <span className={`status ${mine ? (engaged ? 'status-engaged' : '') : 'status-held'}`}>
            {holderLabel(holder, consoleId)}
          </span>
        ) : (
          'nobody'
        )}
      </Row>
      <Row label="since">{control?.since ? timestampDate(control.since).toLocaleTimeString() : '–'}</Row>
      <div className="hand">
        {control?.available.map((controller) => (
          <button
            key={controller.id}
            type="button"
            onClick={() => handTo(controller.id)}
            disabled={holder?.id === controller.id}
          >
            Hand to {controller.name}
          </button>
        ))}
      </div>
      {holder && holder.kind !== ControllerKind.OPERATOR && (
        <p className="hint">Engage to take over; release to hand back.</p>
      )}
      {another && engaged && (
        <p className="hint">Another operator holds the robot; your driving waits until they let go.</p>
      )}
    </section>
  )
}
