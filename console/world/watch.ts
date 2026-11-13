import type { Observation } from '@tourian/contract/tourian/v1/robot_pb'
import { heading, positionOf } from '@/platform/store'
import { openSocket, platformBase } from '@/platform/socket'
import { startWorldView, type Pose } from './view'

// Follows every robot's observations as a console would and renders what one of them sees, as a stream to lend in place of a camera's.
export function watchWorld(robotId: string): { stream: MediaStream; stop: () => void } {
  const view = startWorldView()
  const poses = new Map<string, Pose>()
  const socket = openSocket(platformBase() + '/api/console', {
    onMessage: (envelope) => {
      const m = envelope.message
      if (m.case !== 'observation') return
      const pose = poseOf(m.value)
      if (pose) poses.set(m.value.robotId, pose)
      const own = poses.get(robotId)
      if (own)
        view.update(
          own,
          [...poses.entries()].filter(([id]) => id !== robotId).map(([, p]) => p),
        )
    },
  })
  return {
    stream: view.stream,
    stop: () => {
      socket.close()
      view.stop()
    },
  }
}

function poseOf(observation: Observation): Pose | undefined {
  const position = positionOf(observation)
  return position ? { position, heading: heading(observation) } : undefined
}
