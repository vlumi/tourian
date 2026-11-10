import { create } from '@bufbuild/protobuf'
import { timestampNow } from '@bufbuild/protobuf/wkt'
import { ControlRequestSchema } from '@tourian/contract/tourian/v1/control_pb'
import { HeartbeatSchema, StopSchema, VelocityCommandSchema } from '@tourian/contract/tourian/v1/robot_pb'
import { send } from '@/platform/connection'
import { effectiveLimits, useStore } from '@/platform/store'
import { readGamepad } from './gamepad'
import { twistFromKeys, watchKeyboard } from './keyboard'
import { markSent } from './latency'

export const commandIntervalMs = 50

let sequence = 0n

// Reads the keyboard and the gamepad at the command rate and, while the operator is engaged and the window has focus, sends a heartbeat and a command every tick. Losing focus disengages, so the robot's dead-man switch takes over.
export function startDriver(): () => void {
  const keyboard = watchKeyboard()
  const store = useStore

  const onKey = (event: KeyboardEvent) => {
    if (event.repeat) return
    if (event.code === 'Enter') toggleEngaged()
    if (event.code === 'Space') emergencyStop()
  }
  const onBlur = () => disengage()
  const onVisibility = () => {
    if (document.visibilityState !== 'visible') disengage()
  }
  window.addEventListener('keydown', onKey)
  window.addEventListener('blur', onBlur)
  document.addEventListener('visibilitychange', onVisibility)

  const timer = setInterval(() => {
    const pad = readGamepad()
    if (pad.stopPressed) emergencyStop()
    if (pad.engagePressed) toggleEngaged()
    const keys = twistFromKeys(keyboard.held)
    const input =
      pad.connected && (pad.linear !== 0 || pad.angular !== 0)
        ? { linear: pad.linear, angular: pad.angular, source: 'gamepad' as const }
        : {
            ...keys,
            source:
              keys.linear || keys.angular
                ? ('keyboard' as const)
                : pad.connected
                  ? ('gamepad' as const)
                  : ('none' as const),
          }
    store.getState().setInput(input)

    const { engaged, selected, robots, configs } = store.getState()
    if (!engaged || !selected) return
    const limits = effectiveLimits(robots[selected], configs[selected])
    sendCommand(selected, input.linear * limits.maxLinear, input.angular * limits.maxAngular)
  }, commandIntervalMs)

  return () => {
    clearInterval(timer)
    keyboard.stop()
    window.removeEventListener('keydown', onKey)
    window.removeEventListener('blur', onBlur)
    document.removeEventListener('visibilitychange', onVisibility)
  }
}

export function toggleEngaged(): void {
  if (useStore.getState().engaged) disengage()
  else useStore.getState().engage()
}

// Releasing hands the robot back to whoever held it before the operator took over, if anyone.
function disengage(): void {
  const { engaged, selected } = useStore.getState()
  if (engaged && selected) {
    sendCommand(selected, 0, 0)
    send({ case: 'controlRequest', value: create(ControlRequestSchema, { robotId: selected, release: true }) })
  }
  useStore.getState().disengage()
}

export function handTo(controllerId: string): void {
  const { selected } = useStore.getState()
  if (!selected) return
  useStore.getState().disengage()
  send({ case: 'controlRequest', value: create(ControlRequestSchema, { robotId: selected, controllerId }) })
}

export function emergencyStop(): void {
  const { selected } = useStore.getState()
  if (selected)
    send({ case: 'stop', value: create(StopSchema, { header: { stamp: timestampNow() }, robotId: selected }) })
  useStore.getState().disengage(true)
}

function sendCommand(robotId: string, linear: number, angular: number): void {
  sequence += 1n
  const header = { stamp: timestampNow(), frameId: 'base_link' }
  send({ case: 'heartbeat', value: create(HeartbeatSchema, { header, robotId, sequence }) })
  markSent(sequence)
  send({
    case: 'velocityCommand',
    value: create(VelocityCommandSchema, {
      header,
      robotId,
      sequence,
      twist: { linear: { x: linear }, angular: { z: angular } },
    }),
  })
}
