'use client'

import { timestampDate } from '@bufbuild/protobuf/wkt'
import { ControllerKind, type Controller } from '@tourian/contract/tourian/v1/control_pb'
import { handTo } from '@/drive/driver'
import { useStore } from '@/platform/store'

// Who holds the robot and since when, and the controllers it can be handed to. The operator takes over by engaging and hands back by releasing.
export function ControlPanel() {
  const control = useStore((s) => s.control)
  const engaged = useStore((s) => s.engaged)
  const holder = control?.holder

  return (
    <section className="control">
      <h2>Control</h2>
      <div className="row">
        <span className="label">holder</span>
        <span className="value">
          {holder ? (
            <span
              className={`status ${holder.kind === ControllerKind.OPERATOR ? (engaged ? 'status-engaged' : '') : 'status-held'}`}
            >
              {describe(holder, engaged)}
            </span>
          ) : (
            'nobody'
          )}
        </span>
      </div>
      <div className="row">
        <span className="label">since</span>
        <span className="value">{control?.since ? timestampDate(control.since).toLocaleTimeString() : '–'}</span>
      </div>
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
    </section>
  )
}

function describe(holder: Controller, engaged: boolean): string {
  switch (holder.kind) {
    case ControllerKind.OPERATOR:
      return engaged ? 'you' : 'an operator'
    case ControllerKind.SCRIPT:
      return `script: ${holder.name}`
    case ControllerKind.POLICY:
      return holder.name
    default:
      return holder.name || holder.id
  }
}
