import type { Capabilities, Observation } from '@tourian/contract/tourian/v1/robot_pb'
import { HaltReason } from '@tourian/contract/tourian/v1/robot_pb'
import { create } from 'zustand'

export type Status = 'connecting' | 'connected' | 'disconnected'
export type InputSource = 'none' | 'keyboard' | 'gamepad'

export type Input = { linear: number; angular: number; source: InputSource }

export type Point = { x: number; y: number }

export const historyLength = 120
export const trailLength = 600

export type State = {
  status: Status
  robots: Record<string, Capabilities>
  selected?: string
  observation?: Observation
  trail: Point[]
  history: { speed: number[]; angular: number[]; battery: number[]; latency: number[] }
  latencyMs?: number
  engaged: boolean
  stopped: boolean
  input: Input

  setStatus: (status: Status) => void
  addRobot: (capabilities: Capabilities) => void
  observe: (observation: Observation) => void
  recordLatency: (ms: number) => void
  engage: () => void
  disengage: (stopped?: boolean) => void
  setInput: (input: Input) => void
}

const idle: Input = { linear: 0, angular: 0, source: 'none' }

export const useStore = create<State>()((set) => ({
  status: 'connecting',
  robots: {},
  trail: [],
  history: { speed: [], angular: [], battery: [], latency: [] },
  engaged: false,
  stopped: false,
  input: idle,

  setStatus: (status) =>
    set(status === 'connected' ? { status } : { status, engaged: false, robots: {}, selected: undefined }),
  addRobot: (capabilities) =>
    set((state) => ({
      robots: { ...state.robots, [capabilities.robotId]: capabilities },
      selected: state.selected ?? capabilities.robotId,
    })),
  observe: (observation) =>
    set((state) => {
      if (observation.robotId !== state.selected) return {}
      const position = observation.pose?.position
      const trail = position ? push(state.trail, { x: position.x, y: position.y }, trailLength) : state.trail
      return {
        observation,
        trail,
        history: {
          ...state.history,
          speed: push(state.history.speed, observation.velocity?.linear?.x ?? 0, historyLength),
          angular: push(state.history.angular, observation.velocity?.angular?.z ?? 0, historyLength),
          battery: push(state.history.battery, observation.battery?.percentage ?? 0, historyLength),
        },
      }
    }),
  recordLatency: (ms) =>
    set((state) => ({
      latencyMs: ms,
      history: { ...state.history, latency: push(state.history.latency, ms, historyLength) },
    })),
  engage: () => set({ engaged: true, stopped: false }),
  disengage: (stopped = false) => set({ engaged: false, stopped }),
  setInput: (input) => set({ input }),
}))

function push<T>(list: T[], item: T, limit: number): T[] {
  const next = list.length >= limit ? list.slice(list.length - limit + 1) : list.slice()
  next.push(item)
  return next
}

export function heading(observation?: Observation): number {
  const q = observation?.pose?.orientation
  return q ? 2 * Math.atan2(q.z, q.w) : 0
}

export function haltLabel(reason: HaltReason | undefined): string | undefined {
  switch (reason) {
    case HaltReason.COMMAND_TIMEOUT:
      return 'command timeout'
    case HaltReason.DEAD_MAN:
      return 'dead man'
    case HaltReason.EMERGENCY_STOP:
      return 'emergency stop'
    default:
      return undefined
  }
}
