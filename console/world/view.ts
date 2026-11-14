import * as THREE from 'three'
import type { Point } from '@/platform/store'
import { arenaHalf, landmarks } from './arena'

export type Pose = { position: Point; heading: number }

const eyeHeight = 0.3
const fieldOfView = 70
const robotRadius = 0.25

// What a robot in the arena would see: a camera at its eye height looking along its heading, rendered continuously into a canvas whose stream stands in for a real camera's.
export function startWorldView(width = 640, height = 480) {
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, preserveDrawingBuffer: true })
  renderer.setSize(width, height, false)
  const scene = new THREE.Scene()
  scene.background = new THREE.Color('#0b0f0a')
  scene.fog = new THREE.Fog('#0b0f0a', 8, 16)
  const camera = new THREE.PerspectiveCamera(fieldOfView, width / height, 0.05, 50)
  camera.up.set(0, 0, 1)

  scene.add(new THREE.HemisphereLight('#e6f5c8', '#2a3a20', 2.5))
  const sun = new THREE.DirectionalLight('#ffffff', 2.5)
  sun.position.set(4, -6, 10)
  scene.add(sun)

  const floor = new THREE.Mesh(
    new THREE.PlaneGeometry(2 * arenaHalf + 2, 2 * arenaHalf + 2),
    new THREE.MeshStandardMaterial({ color: '#20291a' }),
  )
  scene.add(floor)
  const grid = new THREE.GridHelper(2 * arenaHalf + 2, 2 * arenaHalf + 2, '#8fd400', '#4a6e10')
  grid.rotation.x = Math.PI / 2
  grid.position.z = 0.005
  scene.add(grid)

  const wall = new THREE.MeshStandardMaterial({ color: '#3d5230' })
  const span = 2 * arenaHalf + 0.2
  for (const [x, y, w, d] of [
    [0, arenaHalf + 0.1, span, 0.2],
    [0, -arenaHalf - 0.1, span, 0.2],
    [arenaHalf + 0.1, 0, 0.2, span],
    [-arenaHalf - 0.1, 0, 0.2, span],
  ]) {
    const mesh = new THREE.Mesh(new THREE.BoxGeometry(w, d, 0.5), wall)
    mesh.position.set(x, y, 0.25)
    scene.add(mesh)
  }
  for (const mark of landmarks) {
    const mesh = new THREE.Mesh(
      new THREE.BoxGeometry(mark.size, mark.size, mark.height),
      new THREE.MeshStandardMaterial({ color: mark.color }),
    )
    mesh.position.set(mark.x, mark.y, mark.height / 2)
    scene.add(mesh)
  }

  const others = new THREE.Group()
  scene.add(others)
  const body = new THREE.CylinderGeometry(robotRadius, robotRadius, 0.3, 24)
  body.rotateX(Math.PI / 2)
  const nose = new THREE.BoxGeometry(0.2, 0.08, 0.08)
  const skin = new THREE.MeshStandardMaterial({ color: '#4a6e10' })
  const robot = (pose: Pose) => {
    const group = new THREE.Group()
    const trunk = new THREE.Mesh(body, skin)
    trunk.position.z = 0.15
    const front = new THREE.Mesh(nose, new THREE.MeshStandardMaterial({ color: '#e6f5c8' }))
    front.position.set(robotRadius, 0, 0.3)
    group.add(trunk, front)
    group.position.set(pose.position.x, pose.position.y, 0)
    group.rotation.z = pose.heading
    return group
  }

  let own: Pose = { position: { x: 0, y: 0 }, heading: 0 }
  let frame = 0
  const paint = () => {
    const { x, y } = own.position
    camera.position.set(x, y, eyeHeight)
    camera.lookAt(x + Math.cos(own.heading), y + Math.sin(own.heading), eyeHeight)
    renderer.render(scene, camera)
    frame = requestAnimationFrame(paint)
  }
  frame = requestAnimationFrame(paint)

  return {
    canvas,
    stream: canvas.captureStream(30),
    update(pose: Pose, seen: Pose[]) {
      own = pose
      others.clear()
      for (const other of seen) others.add(robot(other))
    },
    stop() {
      cancelAnimationFrame(frame)
      renderer.dispose()
    },
  }
}
