import { create } from '@bufbuild/protobuf'
import {
  IceCandidateSchema,
  SessionDescriptionSchema,
  type IceCandidate,
  type SessionDescription,
} from '@tourian/contract/tourian/v1/video_pb'
import type { Message } from '@/platform/codec'

export type PeerLike = Pick<RTCPeerConnection, 'setRemoteDescription' | 'addIceCandidate'> & {
  remoteDescription: RTCSessionDescription | null
}

// Candidates can arrive before the description they belong to; they wait until it is set.
export function remoteSide(pc: PeerLike) {
  const pending: IceCandidate[] = []
  const add = (candidate: IceCandidate) =>
    pc.addIceCandidate({
      candidate: candidate.candidate,
      sdpMid: candidate.sdpMid,
      sdpMLineIndex: candidate.sdpMlineIndex,
    })
  return {
    async setDescription(description: SessionDescription) {
      await pc.setRemoteDescription({ type: description.type as RTCSdpType, sdp: description.sdp })
      for (const candidate of pending.splice(0)) await add(candidate)
    },
    async addCandidate(candidate: IceCandidate) {
      if (pc.remoteDescription) await add(candidate)
      else pending.push(candidate)
    },
  }
}

export function candidateMessage(robotId: string, peerId: string, candidate: RTCIceCandidate): Message {
  return {
    case: 'iceCandidate',
    value: create(IceCandidateSchema, {
      robotId,
      peerId,
      candidate: candidate.candidate,
      sdpMid: candidate.sdpMid ?? '',
      sdpMlineIndex: candidate.sdpMLineIndex ?? 0,
    }),
  }
}

export function descriptionMessage(robotId: string, peerId: string, local: RTCSessionDescription): Message {
  return {
    case: 'sessionDescription',
    value: create(SessionDescriptionSchema, { robotId, peerId, type: local.type, sdp: local.sdp }),
  }
}

export const rtcConfiguration: RTCConfiguration = { iceServers: [] }
