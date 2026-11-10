import { create } from '@bufbuild/protobuf'
import { IceCandidateSchema, SessionDescriptionSchema } from '@tourian/contract/tourian/v1/video_pb'
import { remoteSide, type PeerLike } from './peer'

function fakePeer() {
  const added: RTCIceCandidateInit[] = []
  const pc: PeerLike = {
    remoteDescription: null,
    async setRemoteDescription(description) {
      pc.remoteDescription = description as RTCSessionDescription
    },
    async addIceCandidate(candidate) {
      added.push(candidate as RTCIceCandidateInit)
    },
  }
  return { pc, added }
}

test('candidates wait for the remote description', async () => {
  const { pc, added } = fakePeer()
  const side = remoteSide(pc)
  await side.addCandidate(create(IceCandidateSchema, { candidate: 'early', sdpMid: '0' }))
  expect(added).toEqual([])
  await side.setDescription(create(SessionDescriptionSchema, { type: 'offer', sdp: 'v=0' }))
  expect(added.map((c) => c.candidate)).toEqual(['early'])
  await side.addCandidate(create(IceCandidateSchema, { candidate: 'late', sdpMid: '0' }))
  expect(added.map((c) => c.candidate)).toEqual(['early', 'late'])
})
