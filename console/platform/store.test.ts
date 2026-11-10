import { create } from '@bufbuild/protobuf'
import { CapabilitiesSchema, HaltReason, ObservationSchema } from '@tourian/contract/tourian/v1/robot_pb'
import { ControlStateSchema } from '@tourian/contract/tourian/v1/control_pb'
import { CameraAnnounceSchema } from '@tourian/contract/tourian/v1/video_pb'
import { haltLabel, heading, historyLength, useStore } from './store'

const initial = useStore.getState()

beforeEach(() => useStore.setState(initial, true))

test('the first robot to announce is selected and observed', () => {
  const store = useStore.getState()
  store.addRobot(create(CapabilitiesSchema, { robotId: 'r1', name: 'One' }))
  store.addRobot(create(CapabilitiesSchema, { robotId: 'r2', name: 'Two' }))
  expect(useStore.getState().selected).toBe('r1')

  store.observe(create(ObservationSchema, { robotId: 'r2', pose: { position: { x: 9 } } }))
  expect(useStore.getState().observation).toBeUndefined()

  store.observe(
    create(ObservationSchema, { robotId: 'r1', pose: { position: { x: 1, y: 2 } }, velocity: { linear: { x: 0.5 } } }),
  )
  expect(useStore.getState().trail).toEqual([{ x: 1, y: 2 }])
  expect(useStore.getState().history.speed).toEqual([0.5])
})

test('history keeps the last samples only', () => {
  const store = useStore.getState()
  for (let i = 0; i < historyLength + 5; i++) store.recordLatency(i)
  const latency = useStore.getState().history.latency
  expect(latency).toHaveLength(historyLength)
  expect(latency[0]).toBe(5)
  expect(useStore.getState().latencyMs).toBe(historyLength + 4)
})

test('losing the link releases control and forgets the robots and cameras', () => {
  const store = useStore.getState()
  store.addRobot(create(CapabilitiesSchema, { robotId: 'r1' }))
  store.addCamera(create(CameraAnnounceSchema, { robotId: 'r1', name: 'phone' }))
  expect(useStore.getState().cameras.r1?.name).toBe('phone')
  store.engage()
  store.setStatus('disconnected')
  expect(useStore.getState()).toMatchObject({ engaged: false, robots: {}, cameras: {}, selected: undefined })
})

test('control state is kept for the selected robot only', () => {
  const store = useStore.getState()
  store.addRobot(create(CapabilitiesSchema, { robotId: 'r1' }))
  store.setControl(create(ControlStateSchema, { robotId: 'r2', holder: { id: 'square' } }))
  expect(useStore.getState().control).toBeUndefined()
  store.setControl(create(ControlStateSchema, { robotId: 'r1', holder: { id: 'square' } }))
  expect(useStore.getState().control?.holder?.id).toBe('square')
})

test('heading comes from the yaw quaternion', () => {
  const quarter = create(ObservationSchema, {
    pose: { orientation: { z: Math.sin(Math.PI / 4), w: Math.cos(Math.PI / 4) } },
  })
  expect(heading(quarter)).toBeCloseTo(Math.PI / 2)
  expect(haltLabel(HaltReason.DEAD_MAN)).toBe('dead man')
  expect(haltLabel(HaltReason.UNSPECIFIED)).toBeUndefined()
})
