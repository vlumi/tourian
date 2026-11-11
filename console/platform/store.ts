import type { Capabilities, Observation } from '@tourian/contract/tourian/v1/robot_pb'
import type { RobotConfig } from '@tourian/contract/tourian/v1/config_pb'
import type { ControlState } from '@tourian/contract/tourian/v1/control_pb'
import type { EpisodeStatus } from '@tourian/contract/tourian/v1/episode_pb'
import type { CameraAnnounce } from '@tourian/contract/tourian/v1/video_pb'
import { HaltReason } from '@tourian/contract/tourian/v1/robot_pb'
import { create } from 'zustand'

type Status = 'connecting' | 'connected' | 'disconnected'
type InputSource = 'none' | 'keyboard' | 'gamepad'
export type VideoPath = 'off' | 'webrtc' | 'mjpeg'

export type Input = { linear: number; angular: number; source: InputSource }

export type Point = { x: number; y: number }

export const historyLength = 120
const trailLength = 600

type State = {
  status: Status
  robots: Record<string, Capabilities>
  cameras: Record<string, CameraAnnounce>
  observations: Record<string, Observation>
  configs: Record<string, RobotConfig>
  controls: Record<string, ControlState>
  episodes: Record<string, EpisodeStatus>
  videoPath: VideoPath
  episode?: EpisodeStatus
  episodeBad: boolean
  consoleId?: string
  control?: ControlState
  selected?: string
  observation?: Observation
  trail: Point[]
  history: { speed: number[]; angular: number[]; battery: number[]; latency: number[] }
  latencyMs?: number
  engaged: boolean
  stopped: boolean
  input: Input

  setStatus: (status: Status) => void
  welcome: (consoleId: string) => void
  addRobot: (capabilities: Capabilities) => void
  select: (robotId: string) => void
  addCamera: (announce: CameraAnnounce) => void
  setVideoPath: (path: VideoPath) => void
  setEpisode: (status: EpisodeStatus) => void
  setEpisodeBad: (bad: boolean) => void
  setControl: (state: ControlState) => void
  setConfig: (config: RobotConfig) => void
  observe: (observation: Observation) => void
  recordLatency: (ms: number) => void
  engage: () => void
  disengage: (stopped?: boolean) => void
  setInput: (input: Input) => void
}

const idle: Input = { linear: 0, angular: 0, source: 'none' }
const emptyHistory: State['history'] = { speed: [], angular: [], battery: [], latency: [] }

export const useStore = create<State>()((set) => ({
  status: 'connecting',
  robots: {},
  cameras: {},
  observations: {},
  configs: {},
  controls: {},
  episodes: {},
  videoPath: 'webrtc',
  episodeBad: false,
  trail: [],
  history: emptyHistory,
  engaged: false,
  stopped: false,
  input: idle,

  setStatus: (status) =>
    set(
      status === 'connected'
        ? { status }
        : { status, engaged: false, robots: {}, cameras: {}, selected: undefined, consoleId: undefined },
    ),
  welcome: (consoleId) => set({ consoleId }),
  addRobot: (capabilities) =>
    set((state) => ({
      robots: { ...state.robots, [capabilities.robotId]: capabilities },
      selected: state.selected ?? capabilities.robotId,
    })),
  // Selecting another robot starts its trail and history afresh; what the fleet already knows about it is carried over.
  select: (robotId) =>
    set((state) => ({
      selected: robotId,
      engaged: false,
      stopped: false,
      observation: state.observations[robotId],
      control: state.controls[robotId],
      episode: state.episodes[robotId],
      episodeBad: false,
      trail: [],
      history: emptyHistory,
      latencyMs: undefined,
    })),
  addCamera: (announce) => set((state) => ({ cameras: { ...state.cameras, [announce.robotId]: announce } })),
  setVideoPath: (videoPath) => set({ videoPath }),
  setEpisode: (episode) =>
    set((state) => ({
      episodes: { ...state.episodes, [episode.robotId]: episode },
      ...(episode.robotId === state.selected
        ? { episode, episodeBad: episode.episodeId === state.episode?.episodeId ? state.episodeBad : false }
        : {}),
    })),
  setEpisodeBad: (episodeBad) => set({ episodeBad }),
  setConfig: (config) => set((state) => ({ configs: { ...state.configs, [config.robotId]: config } })),
  setControl: (control) =>
    set((state) => ({
      controls: { ...state.controls, [control.robotId]: control },
      ...(control.robotId === state.selected ? { control } : {}),
    })),
  observe: (observation) =>
    set((state) => {
      const observations = { ...state.observations, [observation.robotId]: observation }
      if (observation.robotId !== state.selected) return { observations }
      const position = positionOf(observation)
      const trail = position ? push(state.trail, position, trailLength) : state.trail
      return {
        observations,
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

export function positionOf(observation?: Observation): Point | undefined {
  const p = observation?.pose?.position
  return p ? { x: p.x, y: p.y } : undefined
}

export function heading(observation?: Observation): number {
  const q = observation?.pose?.orientation
  return q ? 2 * Math.atan2(q.z, q.w) : 0
}

// The limits a robot is driven within: the robot's own, lowered by its configuration where that says so.
export function effectiveLimits(
  capabilities?: Capabilities,
  config?: RobotConfig,
): { maxLinear: number; maxAngular: number } {
  const cap = capabilities?.velocityLimits
  const cfg = config?.limits
  const lower = (a: number | undefined, b: number | undefined) => {
    const candidates = [a, b].filter((v): v is number => v !== undefined && v > 0)
    return candidates.length ? Math.min(...candidates) : 0
  }
  return { maxLinear: lower(cap?.maxLinear, cfg?.maxLinear), maxAngular: lower(cap?.maxAngular, cfg?.maxAngular) }
}

export function batteryLabel(observation: Observation | undefined, digits = 1): string {
  return observation?.battery ? `${(observation.battery.percentage * 100).toFixed(digits)} %` : '–'
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
